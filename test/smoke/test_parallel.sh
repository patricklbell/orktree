#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

$ORKTREE switch branch-a
$ORKTREE switch branch-b

ws_a=$($ORKTREE path branch-a)
ws_b=$($ORKTREE path branch-b)

# Run two background processes in parallel
(
  cd "$ws_a"
  echo "change-a" > parallel_a.txt
  git add parallel_a.txt
  git commit -m "commit on branch-a"
) &
pid_a=$!

(
  cd "$ws_b"
  echo "change-b" > parallel_b.txt
  git add parallel_b.txt
  git commit -m "commit on branch-b"
) &
pid_b=$!

wait $pid_a
wait $pid_b

# Isolation checks
assert_file_exists "$ws_a/parallel_a.txt"
assert_file_not_exists "$ws_b/parallel_a.txt"
assert_file_exists "$ws_b/parallel_b.txt"
assert_file_not_exists "$ws_a/parallel_b.txt"

# Merge branch-a into source root
cd "$REPO_DIR"
git merge branch-a --no-edit
assert_file_exists "$REPO_DIR/parallel_a.txt"

# Clean up
$ORKTREE rm branch-a --force
$ORKTREE rm branch-b --force
