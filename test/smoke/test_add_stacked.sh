#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

base_ws=$("$ORKTREE" add ../base-branch)

echo "base content" > "$base_ws/base_file.txt"

stacked_ws=$("$ORKTREE" add ../stacked base-branch)

# CoW lowerdir: base_file.txt should be visible in stacked workspace
assert_file_exists "$stacked_ws/base_file.txt"

# Isolation: writes in stacked should not appear in base
echo "stacked content" > "$stacked_ws/stacked_file.txt"
assert_file_not_exists "$base_ws/stacked_file.txt"
