#!/usr/bin/env bash
set -euo pipefail

warden_state_dir() {
  printf '%s' "${WARDEN_STATE_DIR:-${HOME}/.orktree-warden}"
}

repo_root() {
  if [[ $# -gt 0 && -n "${1}" ]]; then
    printf '%s' "$1"
    return
  fi
  git rev-parse --show-toplevel
}

repo_key() {
  local root="$1"
  printf '%s' "$root" | sha1sum | cut -c1-16
}

runs_dir() {
  local root="$1"
  local key
  key=$(repo_key "$root")
  printf '%s/runs/%s' "$(warden_state_dir)" "$key"
}

ensure_prereqs() {
  local required=(docker jq orktree sha1sum)
  local cmd
  for cmd in "${required[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "warden: missing required command: $cmd" >&2
      exit 1
    fi
  done
}

cpu_parallelism() {
  if [[ -n "${WARDEN_MAX_PARALLEL:-}" ]]; then
    printf '%s' "$WARDEN_MAX_PARALLEL"
    return
  fi
  if command -v nproc >/dev/null 2>&1; then
    nproc
    return
  fi
  getconf _NPROCESSORS_ONLN
}

run_id() {
  local seed
  seed="$(date +%s%N)-${RANDOM}-${RANDOM}-$$"
  printf '%s' "$seed" | sha1sum | cut -c1-12
}

json_escape() {
  jq -R -s '.'
}

usage_error() {
  local message="$1"
  local usage="$2"
  echo "$message" >&2
  echo "$usage" >&2
  exit 2
}