#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup

# Build a minimal submodule repo inside the temp dir
SUB_DIR="$SMOKE_TMPDIR/submod"
mkdir -p "$SUB_DIR"
git -C "$SUB_DIR" init -b main
git -C "$SUB_DIR" config user.email "smoke@test"
git -C "$SUB_DIR" config user.name "Smoke Test"
echo "sub" > "$SUB_DIR/sub.txt"
git -C "$SUB_DIR" add sub.txt
git -C "$SUB_DIR" commit -m "sub initial"

# Add the submodule to the main repo
git -C "$REPO_DIR" -c protocol.file.allow=always submodule add "$SUB_DIR" submod
git -C "$REPO_DIR" commit -m "add submodule"

cd "$REPO_DIR"
ws=$("$ORKTREE" add ../feature-sub)

assert_dir_exists "$ws"
assert_branch "$ws" "feature-sub"

# git status must succeed (fails if submodule .git gitfile paths are unresolvable)
git -C "$ws" status || fail "git status failed in orktree workspace with submodule"

# The submodule directory should be visible through the overlay
assert_dir_exists "$ws/submod"
