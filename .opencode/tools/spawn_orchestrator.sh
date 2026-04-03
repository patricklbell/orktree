#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

TASK=""
LABEL=""
ROOT=""
AGENT="orchestrator"
TTL_SECONDS="${WARDEN_RUN_TTL_SECONDS:-14400}"

usage() {
  cat <<'EOF'
usage: spawn_orchestrator.sh --task <task> [--label <label>] [--repo-root <path>] [--agent <name>] [--ttl-seconds <seconds>]

Creates one isolated orktree+container run and starts the requested OpenCode agent.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --task)
      TASK="$2"
      shift 2
      ;;
    --label)
      LABEL="$2"
      shift 2
      ;;
    --repo-root)
      ROOT="$2"
      shift 2
      ;;
    --agent)
      AGENT="$2"
      shift 2
      ;;
    --ttl-seconds)
      TTL_SECONDS="$2"
      shift 2
      ;;
    *)
      usage_error "spawn_orchestrator: unknown argument: $1" "$(usage)"
      ;;
  esac
done

if [[ -z "$TASK" ]]; then
  usage_error "spawn_orchestrator: --task is required" "$(usage)"
fi

if ! [[ "$TTL_SECONDS" =~ ^[0-9]+$ ]] || [[ "$TTL_SECONDS" -eq 0 ]]; then
  usage_error "spawn_orchestrator: --ttl-seconds must be a positive integer" "$(usage)"
fi

ensure_prereqs

ROOT=$(repo_root "$ROOT")
STATE_DIR=$(warden_state_dir)
RUNS_DIR=$(runs_dir "$ROOT")
mkdir -p "$RUNS_DIR"
mkdir -p "${STATE_DIR}/locks"

RID=$(run_id)
if [[ -n "$LABEL" ]]; then
  SAFE_LABEL=$(printf '%s' "$LABEL" | tr -cs '[:alnum:]-' '-')
  RID="${RID}-${SAFE_LABEL}"
fi

BRANCH="warden/${RID}"
CONTAINER_NAME="warden-${RID}"

WORKSPACE=$(cd "$ROOT" && orktree path "$BRANCH" 2>&1 | tail -1)
if [[ -z "$WORKSPACE" || ! -d "$WORKSPACE" ]]; then
  echo "spawn_orchestrator: failed to create orktree for branch ${BRANCH}" >&2
  exit 1
fi

SIBLING_DIR="${ROOT}.orktree"
STATE_JSON="${SIBLING_DIR}/state.json"
ORKTREE_ID=$(jq -r --arg b "$BRANCH" '.orktrees[] | select(.branch == $b) | .id' "$STATE_JSON")
if [[ -z "$ORKTREE_ID" || "$ORKTREE_ID" == "null" ]]; then
  echo "spawn_orchestrator: could not find orktree metadata for ${BRANCH}" >&2
  exit 1
fi

GIT_TREE_DIR="${SIBLING_DIR}/.overlayfs/${ORKTREE_ID}/tree"
if [[ ! -d "$GIT_TREE_DIR" ]]; then
  echo "spawn_orchestrator: missing git tree dir ${GIT_TREE_DIR}" >&2
  exit 1
fi

IMAGE_NAME="${WARDEN_IMAGE_NAME:-orktree-warden}"
DOCKERFILE="${WARDEN_DOCKERFILE:-${ROOT}/.devcontainer/Dockerfile}"
if [[ ! -f "$DOCKERFILE" ]]; then
  echo "spawn_orchestrator: dockerfile not found at ${DOCKERFILE}" >&2
  exit 1
fi

LOCK_FILE="${STATE_DIR}/locks/image-build.lock"
if command -v flock >/dev/null 2>&1; then
  exec 9>"$LOCK_FILE"
  flock 9
fi
if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  docker build -q -t "$IMAGE_NAME" -f "$DOCKERFILE" "$ROOT" >/dev/null
fi
if command -v flock >/dev/null 2>&1; then
  flock -u 9
fi

TASK_B64=$(printf '%s' "$TASK" | base64 -w0)
RUN_SCRIPT="set -euo pipefail; TASK=\$(printf '%s' '${TASK_B64}' | base64 -d); if ! command -v opencode >/dev/null 2>&1; then echo 'opencode binary not present in container image' >&2; exit 127; fi; opencode run --config .opencode/opencode.yaml --agent '${AGENT}' --prompt \"\$TASK\""

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
CONTAINER_ID=$(docker run -d \
  --name "$CONTAINER_NAME" \
  --read-only \
  --tmpfs /tmp \
  --tmpfs /run \
  --network none \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  -e HOME=/tmp/warden-home \
  -e OPENCODE_DISABLE_KEYRING=1 \
  -v "${WORKSPACE}:${WORKSPACE}" \
  -v "${ROOT}/.git:${ROOT}/.git" \
  -v "${GIT_TREE_DIR}:${GIT_TREE_DIR}" \
  -w "${WORKSPACE}" \
  "$IMAGE_NAME" \
  bash -lc "$RUN_SCRIPT")

CREATED_AT=$(date +%s)
RUN_FILE="${RUNS_DIR}/${RID}.json"
RUN_FILE_TMP="${RUN_FILE}.tmp.${$}"

cat >"$RUN_FILE_TMP" <<EOF
{
  "run_id": "${RID}",
  "branch": "${BRANCH}",
  "task": $(printf '%s' "$TASK" | json_escape),
  "repo_root": "${ROOT}",
  "workspace_path": "${WORKSPACE}",
  "container_name": "${CONTAINER_NAME}",
  "container_id": "${CONTAINER_ID}",
  "created_at": ${CREATED_AT},
  "ttl_seconds": ${TTL_SECONDS},
  "preserve_on_finish": true
}
EOF
mv "$RUN_FILE_TMP" "$RUN_FILE"

jq -n \
  --arg run_id "$RID" \
  --arg branch "$BRANCH" \
  --arg container "$CONTAINER_NAME" \
  --arg workspace "$WORKSPACE" \
  --argjson max_parallel "$(cpu_parallelism)" \
  '{run_id:$run_id, branch:$branch, container_name:$container, workspace_path:$workspace, status:"running", max_parallel:$max_parallel}'