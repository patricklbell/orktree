#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

RUN_ID=""
ROOT=""
KEEP_ORKTREE="${WARDEN_KEEP_ORKTREE_ON_CLEANUP:-0}"

usage() {
  cat <<'EOF'
usage: cleanup_run.sh --run-id <id> [--repo-root <path>] [--keep-orktree]

Removes one orchestrator run's container and, by default, its temporary orktree.
Set WARDEN_KEEP_ORKTREE_ON_CLEANUP=1 (or pass --keep-orktree) to preserve the orktree.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --run-id)
      RUN_ID="$2"
      shift 2
      ;;
    --repo-root)
      ROOT="$2"
      shift 2
      ;;
    --keep-orktree)
      KEEP_ORKTREE="1"
      shift
      ;;
    *)
      usage_error "cleanup_run: unknown argument: $1" "$(usage)"
      ;;
  esac
done

if [[ -z "$RUN_ID" ]]; then
  usage_error "cleanup_run: --run-id is required" "$(usage)"
fi

ensure_prereqs
ROOT=$(repo_root "$ROOT")
RUNS_DIR=$(runs_dir "$ROOT")
RUN_FILE="${RUNS_DIR}/${RUN_ID}.json"

if [[ ! -f "$RUN_FILE" ]]; then
  jq -n --arg run_id "$RUN_ID" '{run_id:$run_id,status:"missing"}'
  exit 0
fi

CONTAINER_NAME=$(jq -r '.container_name' "$RUN_FILE")
BRANCH=$(jq -r '.branch' "$RUN_FILE")

docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

if [[ "$KEEP_ORKTREE" != "1" ]]; then
  (cd "$ROOT" && orktree rm "$BRANCH" --force >/dev/null 2>&1) || true
fi

rm -f "$RUN_FILE"

jq -n --arg run_id "$RUN_ID" --arg branch "$BRANCH" --arg keep_orktree "$KEEP_ORKTREE" '{run_id:$run_id, branch:$branch, status:"cleaned", keep_orktree:($keep_orktree=="1")}'