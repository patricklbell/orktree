#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

$ORKTREE switch feature-x

ws=$($ORKTREE path feature-x)
assert_dir_exists "$ws"

echo "hello" > "$ws/test_file.txt"
assert_file_exists "$ws/test_file.txt"

assert_branch "$ws" "feature-x"
