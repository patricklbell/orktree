#!/usr/bin/env bash
# Shared helpers for orktree smoke tests. Source this, don't execute it.
set -euo pipefail

ORKTREE="${ORKTREE_BIN:-$(cd "$(dirname "$0")/../.." && pwd)/build/orktree}"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

assert_file_exists() {
  [[ -f "$1" ]] || fail "expected file to exist: $1"
}

assert_file_not_exists() {
  [[ ! -f "$1" ]] || fail "expected file NOT to exist: $1"
}

assert_dir_exists() {
  [[ -d "$1" ]] || fail "expected directory to exist: $1"
}

assert_output_contains() {
  local pattern="$1"; shift
  local output
  output=$("$@" 2>&1) || true
  echo "$output" | grep -qE "$pattern" || fail "output of '$*' did not contain '$pattern'. Got: $output"
}

assert_branch() {
  local dir="$1" branch="$2"
  local actual
  actual=$(git -C "$dir" branch --show-current)
  [[ "$actual" == "$branch" ]] || fail "expected branch '$branch' in $dir, got '$actual'"
}

smoke_teardown() {
  if [[ -n "${SMOKE_TMPDIR:-}" ]]; then
    # Best-effort unmount any fuse-overlayfs mounts under the temp dir
    mount | grep "$SMOKE_TMPDIR" | awk '{print $3}' | while read -r mp; do
      fusermount -uz "$mp" 2>/dev/null || fusermount3 -uz "$mp" 2>/dev/null || true
    done || true
    rm -rf "$SMOKE_TMPDIR"
  fi
}

smoke_setup() {
  SMOKE_TMPDIR=$(mktemp -d)
  trap smoke_teardown EXIT

  REPO_DIR="$SMOKE_TMPDIR/repo"
  mkdir -p "$REPO_DIR"
  git -C "$REPO_DIR" init -b main
  git -C "$REPO_DIR" config user.email "smoke@test"
  git -C "$REPO_DIR" config user.name "Smoke Test"
  echo "initial" > "$REPO_DIR/README.md"
  git -C "$REPO_DIR" add README.md
  git -C "$REPO_DIR" commit -m "initial commit"

  export SMOKE_TMPDIR REPO_DIR ORKTREE
}
