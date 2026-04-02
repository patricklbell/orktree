#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

"$ORKTREE" switch feature-x
"$ORKTREE" switch feature-x-variant --from feature-x
"$ORKTREE" ls
"$ORKTREE" switch -
"$ORKTREE" rm feature-x-variant
"$ORKTREE" rm feature-x
