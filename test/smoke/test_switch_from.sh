#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

"$ORKTREE" switch base-branch
base_ws=$("$ORKTREE" path base-branch)

echo "base content" > "$base_ws/base_file.txt"

"$ORKTREE" switch stacked --from base-branch
stacked_ws=$("$ORKTREE" path stacked)

# CoW lowerdir: base_file.txt should be visible in stacked workspace
assert_file_exists "$stacked_ws/base_file.txt"

# Isolation: writes in stacked should not appear in base
echo "stacked content" > "$stacked_ws/stacked_file.txt"
assert_file_not_exists "$base_ws/stacked_file.txt"
