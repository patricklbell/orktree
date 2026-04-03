#!/usr/bin/env bash
# Creates an orktree branch and starts an ephemeral Docker container whose
# workspace is the orktree merged view.  Git operations inside the container
# work because the source root's .git and the worktree metadata are mounted at
# their original absolute paths, while the source root's *files* are never
# exposed — overlayfs on the host provides them read-only through the merged
# view.
set -euo pipefail

# ---------------------------------------------------------------------------
# Parse hook input
# ---------------------------------------------------------------------------
INPUT=$(cat)
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd')
TRANSCRIPT=$(printf '%s' "$INPUT" | jq -r '.transcript_path')

# ---------------------------------------------------------------------------
# State directory — one JSON file per active session, keyed by transcript path.
# sessionId is not populated on SessionStart; transcript_path is stable and
# shared between SessionStart and Stop for the same session.
# ---------------------------------------------------------------------------
STATE_DIR="${HOME}/.orktree-fleet"
mkdir -p "$STATE_DIR"
# Hash the transcript path to a short key safe for filenames.
STATE_KEY=$(printf '%s' "$TRANSCRIPT" | sha1sum | cut -c1-16)
STATE_FILE="${STATE_DIR}/${STATE_KEY}.json"

# ---------------------------------------------------------------------------
# Human-readable name generator (adjective-noun, same style as Docker)
# ---------------------------------------------------------------------------
random_name() {
  local adjectives=(amber azure brave brisk calm clever crisp deft eager fair
    fleet golden hardy keen lucid nimble noble quiet rapid sharp
    sleek smart solid stern swift vivid warm wise)
  local nouns=(badger bear condor crane eagle falcon finch fox hawk heron
    ibis kite lark lynx mink otter raven robin swift teal
    viper weasel wolf wren)
  local adj="${adjectives[RANDOM % ${#adjectives[@]}]}"
  local noun="${nouns[RANDOM % ${#nouns[@]}]}"
  printf '%s-%s' "$adj" "$noun"
}

emit_context() {
  local ws="$1" branch="$2"
  # Expose only what agents need: where to work and what branch to integrate into.
  # Infrastructure details (Docker, overlayfs) are intentionally omitted.
  cat <<ENDJSON
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "Working directory: \`${ws}\`\nIntegration branch: \`${branch}\` — merge all task branches into this branch before finishing."
  }
}
ENDJSON
}

# Idempotent — if already provisioned, return cached state.
if [[ -f "$STATE_FILE" ]]; then
  CONTAINER_NAME=$(jq -r '.container_name' "$STATE_FILE")
  WORKSPACE=$(jq -r '.workspace_path' "$STATE_FILE")
  BRANCH=$(jq -r '.branch' "$STATE_FILE")
  # Make sure the container is still running.
  running_state=$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null || true)
  if [[ "$running_state" == "true" ]]; then
    emit_context "$WORKSPACE" "$BRANCH"
    exit 0
  fi
  # Container gone — clean state and re-provision below.
  rm -f "$STATE_FILE"
fi

NAME=$(random_name)
BRANCH="fleet/${NAME}"
CONTAINER_NAME="fleet-${NAME}"

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
for cmd in docker jq orktree; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "fleet-provision: $cmd is required but not found" >&2
    exit 1
  fi
done

# ---------------------------------------------------------------------------
# Create orktree (idempotent)
# ---------------------------------------------------------------------------
cd "$CWD"
WORKSPACE=$(orktree path "$BRANCH" 2>&1 | tail -1)

if [[ -z "$WORKSPACE" || ! -d "$WORKSPACE" ]]; then
  echo "fleet-provision: failed to create orktree for branch $BRANCH" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Derive git paths from state.json
# ---------------------------------------------------------------------------
SOURCE_ROOT="$CWD"
SIBLING_DIR="${SOURCE_ROOT}.orktree"
STATE_JSON="${SIBLING_DIR}/state.json"

ORKTREE_ID=$(jq -r --arg b "$BRANCH" \
  '.orktrees[] | select(.branch == $b) | .id' "$STATE_JSON")

GIT_TREE_DIR="${SIBLING_DIR}/.overlayfs/${ORKTREE_ID}/tree"

# ---------------------------------------------------------------------------
# Build the fleet Docker image (idempotent, cached after first build)
# ---------------------------------------------------------------------------
IMAGE_NAME="orktree-fleet"
if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  docker build -q -t "$IMAGE_NAME" \
    -f "${CWD}/.devcontainer/Dockerfile" "$CWD" >/dev/null 2>&1
fi

# ---------------------------------------------------------------------------
# Start container
#
# Mounts (all at their host absolute paths so gitdir pointers resolve):
#   $WORKSPACE       (rw)  — overlayfs merged view, the working directory
#   $SOURCE_ROOT/.git (rw) — shared git object store & refs (needed for commits)
#   $GIT_TREE_DIR    (rw)  — worktree metadata
#
# The source root's *files* are never mounted — the agent can only reach them
# through the overlayfs merged view, which routes writes to the CoW upper layer.
# ---------------------------------------------------------------------------

# Remove leftover container with the same name (from a crashed session).
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

CONTAINER_ID=$(docker run -d \
  --name "$CONTAINER_NAME" \
  --device=/dev/fuse \
  --cap-add=SYS_ADMIN \
  -v "${WORKSPACE}:${WORKSPACE}" \
  -v "${SOURCE_ROOT}/.git:${SOURCE_ROOT}/.git" \
  -v "${GIT_TREE_DIR}:${GIT_TREE_DIR}" \
  -w "${WORKSPACE}" \
  "$IMAGE_NAME" \
  sleep infinity)

# ---------------------------------------------------------------------------
# Persist fleet state
# ---------------------------------------------------------------------------
cat > "$STATE_FILE" <<EOF
{
  "state_key": "${STATE_KEY}",
  "container_id": "${CONTAINER_ID}",
  "container_name": "${CONTAINER_NAME}",
  "workspace_path": "${WORKSPACE}",
  "branch": "${BRANCH}",
  "source_root": "${SOURCE_ROOT}",
  "git_tree_dir": "${GIT_TREE_DIR}"
}
EOF

emit_context "$WORKSPACE" "$BRANCH"
