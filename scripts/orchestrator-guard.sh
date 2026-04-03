#!/usr/bin/env bash
# Intercepts every tool call during a fleet session and:
#   • Rewrites file paths from the source root to the orktree workspace
#   • Wraps terminal commands in `docker exec` so they run inside the container
#   • Blocks file operations targeting paths outside the orktree
#
# For non-fleet sessions (no state file) the script exits immediately with no
# output, adding negligible overhead.
set -euo pipefail

INPUT=$(cat)
TRANSCRIPT=$(printf '%s' "$INPUT" | jq -r '.transcript_path')

# ---------------------------------------------------------------------------
# Fast-path: not a fleet session → allow everything unchanged
# ---------------------------------------------------------------------------
STATE_DIR="${HOME}/.orktree-fleet"
STATE_KEY=$(printf '%s' "$TRANSCRIPT" | sha1sum | cut -c1-16)
STATE_FILE="${STATE_DIR}/${STATE_KEY}.json"

if [[ ! -f "$STATE_FILE" ]]; then
  exit 0
fi

# ---------------------------------------------------------------------------
# Load fleet state
# ---------------------------------------------------------------------------
WORKSPACE=$(jq -r '.workspace_path' "$STATE_FILE")
SOURCE_ROOT=$(jq -r '.source_root' "$STATE_FILE")
CONTAINER_NAME=$(jq -r '.container_name' "$STATE_FILE")

TOOL_NAME=$(printf '%s' "$INPUT" | jq -r '.tool_name')
TOOL_INPUT=$(printf '%s' "$INPUT" | jq -c '.tool_input')

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# Rewrite a single absolute path.  Returns the rewritten path on stdout.
# Exits with code 2 (blocking error) if the path is outside allowed areas.
rewrite_path() {
  local p="$1"
  # Already inside the orktree workspace — pass through.
  if [[ "$p" == "${WORKSPACE}"* ]]; then
    printf '%s' "$p"
    return
  fi
  # Under the source root — redirect to the orktree.
  if [[ "$p" == "${SOURCE_ROOT}"* ]]; then
    printf '%s' "${WORKSPACE}${p#"${SOURCE_ROOT}"}"
    return
  fi
  # Relative path — prefix with workspace.
  if [[ "$p" != /* ]]; then
    printf '%s' "${WORKSPACE}/${p}"
    return
  fi
  # Absolute path outside allowed areas — deny.
  echo "fleet-guard: path $p is outside the orktree workspace" >&2
  return 1
}

# Emit a PreToolUse "allow" response with updatedInput.
emit_allow() {
  local updated_input="$1"
  jq -n --argjson ui "$updated_input" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "allow",
      updatedInput: $ui
    }
  }'
}

# Emit a PreToolUse "deny" response.
emit_deny() {
  local reason="$1"
  jq -n --arg r "$reason" '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: $r
    }
  }'
}

# ---------------------------------------------------------------------------
# Tool-specific interception
# ---------------------------------------------------------------------------
case "$TOOL_NAME" in

  # --- Single filePath field -------------------------------------------
  create_file|replace_string_in_file|read_file)
    fp=$(printf '%s' "$TOOL_INPUT" | jq -r '.filePath // ""')
    if [[ -z "$fp" ]]; then exit 0; fi
    new_fp=$(rewrite_path "$fp") || {
      emit_deny "Path $fp is outside the fleet workspace"
      exit 0
    }
    updated=$(printf '%s' "$TOOL_INPUT" | jq --arg p "$new_fp" '.filePath = $p')
    emit_allow "$updated"
    ;;

  # --- list_dir uses .path -------------------------------------------
  list_dir)
    fp=$(printf '%s' "$TOOL_INPUT" | jq -r '.path // ""')
    if [[ -z "$fp" ]]; then exit 0; fi
    new_fp=$(rewrite_path "$fp") || {
      emit_deny "Path $fp is outside the fleet workspace"
      exit 0
    }
    updated=$(printf '%s' "$TOOL_INPUT" | jq --arg p "$new_fp" '.path = $p')
    emit_allow "$updated"
    ;;

  # --- multi_replace_string_in_file: array of {filePath, ...} ----------
  multi_replace_string_in_file)
    # Rewrite every replacements[].filePath
    updated="$TOOL_INPUT"
    count=$(printf '%s' "$TOOL_INPUT" | jq '.replacements | length')
    for (( i=0; i<count; i++ )); do
      fp=$(printf '%s' "$updated" | jq -r ".replacements[$i].filePath // \"\"")
      if [[ -z "$fp" ]]; then continue; fi
      new_fp=$(rewrite_path "$fp") || {
        emit_deny "Path $fp is outside the fleet workspace"
        exit 0
      }
      updated=$(printf '%s' "$updated" | jq --arg p "$new_fp" ".replacements[$i].filePath = \$p")
    done
    emit_allow "$updated"
    ;;

  # --- get_errors: array of filePaths ---------------------------------
  get_errors)
    has_paths=$(printf '%s' "$TOOL_INPUT" | jq 'has("filePaths")')
    if [[ "$has_paths" != "true" ]]; then exit 0; fi
    updated="$TOOL_INPUT"
    count=$(printf '%s' "$TOOL_INPUT" | jq '.filePaths | length')
    for (( i=0; i<count; i++ )); do
      fp=$(printf '%s' "$updated" | jq -r ".filePaths[$i]")
      new_fp=$(rewrite_path "$fp") || {
        emit_deny "Path $fp is outside the fleet workspace"
        exit 0
      }
      updated=$(printf '%s' "$updated" | jq --arg p "$new_fp" ".filePaths[$i] = \$p")
    done
    emit_allow "$updated"
    ;;

  # --- Terminal commands: wrap in docker exec -------------------------
  run_in_terminal)
    cmd=$(printf '%s' "$TOOL_INPUT" | jq -r '.command // ""')
    if [[ -z "$cmd" ]]; then exit 0; fi
    # Escape for bash -c inside docker exec.
    wrapped="docker exec -w '${WORKSPACE}' '${CONTAINER_NAME}' bash -c $(printf '%q' "$cmd")"
    updated=$(printf '%s' "$TOOL_INPUT" | jq --arg c "$wrapped" '.command = $c')
    emit_allow "$updated"
    ;;

  # --- Search tools: rewrite absolute includePattern if present -------
  grep_search)
    inc=$(printf '%s' "$TOOL_INPUT" | jq -r '.includePattern // ""')
    if [[ -n "$inc" && "$inc" == "${SOURCE_ROOT}"* ]]; then
      new_inc="${WORKSPACE}${inc#"${SOURCE_ROOT}"}"
      updated=$(printf '%s' "$TOOL_INPUT" | jq --arg p "$new_inc" '.includePattern = $p')
      emit_allow "$updated"
    fi
    # Relative globs are fine — VS Code resolves them against the workspace.
    ;;

  file_search)
    q=$(printf '%s' "$TOOL_INPUT" | jq -r '.query // ""')
    if [[ -n "$q" && "$q" == "${SOURCE_ROOT}"* ]]; then
      new_q="${WORKSPACE}${q#"${SOURCE_ROOT}"}"
      updated=$(printf '%s' "$TOOL_INPUT" | jq --arg p "$new_q" '.query = $p')
      emit_allow "$updated"
    fi
    ;;

  # --- Everything else: allow unchanged (memory, todo, subagent, ...) --
  *)
    exit 0
    ;;

esac
