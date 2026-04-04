#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

ws=$("$ORKTREE" add ../to-remove)
assert_dir_exists "$ws"

"$ORKTREE" rm to-remove --ignore-untracked

output=$("$ORKTREE" ls --quiet 2>&1) || true
if echo "$output" | grep -q "to-remove"; then
  fail "orktree 'to-remove' still listed after rm"
fi
