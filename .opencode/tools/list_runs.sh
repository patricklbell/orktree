#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

ROOT=""

usage() {
  cat <<'EOF'
usage: list_runs.sh [--repo-root <path>]

Lists run metadata and current container status for this repository.
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
    *)
      usage_error "list_runs: unknown argument: $1" "$(usage)"
      ;;
  esac
done

ensure_prereqs
ROOT=$(repo_root "$ROOT")
RUNS_DIR=$(runs_dir "$ROOT")
mkdir -p "$RUNS_DIR"

out='[]'
shopt -s nullglob
for run_file in "${RUNS_DIR}"/*.json; do
  container_name=$(jq -r '.container_name' "$run_file")
  status="finished"
  if docker inspect -f '{{.State.Running}}' "$container_name" >/dev/null 2>&1; then
    status="running"
  fi
  row=$(jq --arg status "$status" '. + {status:$status}' "$run_file")
  out=$(jq --argjson row "$row" '. + [$row]' <<<"$out")
done

jq -n --argjson runs "$out" --argjson max_parallel "$(cpu_parallelism)" '{max_parallel:$max_parallel, runs:$runs}'