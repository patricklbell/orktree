#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

ROOT=""
NOW=$(date +%s)
REAP_FINISHED="${WARDEN_REAP_FINISHED:-0}"

usage() {
  cat <<'EOF'
usage: reap_stale_runs.sh [--repo-root <path>] [--reap-finished]

Removes stale runs and their temporary resources.
By default, finished runs are preserved until TTL expiry to keep workflows lossless.
Use --reap-finished (or WARDEN_REAP_FINISHED=1) for eager cleanup.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --repo-root)
      ROOT="$2"
      shift 2
      ;;
    --reap-finished)
      REAP_FINISHED="1"
      shift
      ;;
    *)
      usage_error "reap_stale_runs: unknown argument: $1" "$(usage)"
      ;;
  esac
done

ensure_prereqs
ROOT=$(repo_root "$ROOT")
RUNS_DIR=$(runs_dir "$ROOT")
mkdir -p "$RUNS_DIR"

cleaned=0
kept=0
finished_preserved=0

shopt -s nullglob
for run_file in "${RUNS_DIR}"/*.json; do
  run_id=$(jq -r '.run_id' "$run_file")
  container_name=$(jq -r '.container_name' "$run_file")
  created_at=$(jq -r '.created_at' "$run_file")
  ttl_seconds=$(jq -r '.ttl_seconds // 14400' "$run_file")
  branch=$(jq -r '.branch' "$run_file")

  expired=0
  if [[ $((NOW - created_at)) -ge "$ttl_seconds" ]]; then
    expired=1
  fi

  running=0
  running_state=$(docker inspect -f '{{.State.Running}}' "$container_name" 2>/dev/null || true)
  if [[ "$running_state" == "true" ]]; then
    running=1
  fi

  should_clean=0
  if [[ "$expired" == "1" ]]; then
    should_clean=1
  elif [[ "$running" == "0" ]]; then
    preserve_on_finish=$(jq -r '.preserve_on_finish // true' "$run_file")
    if [[ "$REAP_FINISHED" == "1" || "$preserve_on_finish" != "true" ]]; then
      should_clean=1
    else
      finished_preserved=$((finished_preserved + 1))
    fi
  fi

  if [[ "$should_clean" == "1" ]]; then
    docker rm -f "$container_name" >/dev/null 2>&1 || true
    (cd "$ROOT" && orktree rm "$branch" --force >/dev/null 2>&1) || true
    rm -f "$run_file"
    cleaned=$((cleaned + 1))
  else
    kept=$((kept + 1))
  fi
done

jq -n \
  --argjson cleaned "$cleaned" \
  --argjson kept "$kept" \
  --argjson finished_preserved "$finished_preserved" \
  '{cleaned:$cleaned, kept:$kept, finished_preserved:$finished_preserved}'