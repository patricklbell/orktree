#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

ws=$("$ORKTREE" add ../mount-test)
assert_dir_exists "$ws"

# Should be mounted after add
assert_file_exists "$ws/README.md"

# Unmount
"$ORKTREE" unmount mount-test

# Re-mount
"$ORKTREE" mount mount-test

# Should be accessible again
assert_file_exists "$ws/README.md"
