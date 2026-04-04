#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

# Test that flags after -- are forwarded to git worktree add.
# Use -b to create a worktree with a custom branch name.
ws=$("$ORKTREE" add ../custom-branch -- -b my-custom-branch)
assert_dir_exists "$ws"
assert_branch "$ws" "my-custom-branch"
