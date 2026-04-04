#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

"$ORKTREE" add ../branch-one
"$ORKTREE" add ../branch-two

assert_output_contains "branch-one" "$ORKTREE" ls
assert_output_contains "branch-two" "$ORKTREE" ls

assert_output_contains "branch-one" "$ORKTREE" ls --quiet
assert_output_contains "branch-two" "$ORKTREE" ls --quiet
