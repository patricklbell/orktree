#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

ws=$("$ORKTREE" add ../move-test)
echo "content" > "$ws/test_file.txt"

"$ORKTREE" move move-test ../moved-test

new_ws=$("$ORKTREE" path move-test)
assert_dir_exists "$new_ws"
assert_file_exists "$new_ws/test_file.txt"
assert_file_exists "$new_ws/README.md"
