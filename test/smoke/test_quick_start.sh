#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/lib.sh"
smoke_setup
cd "$REPO_DIR"

"$ORKTREE" add ../feature-x
"$ORKTREE" add ../feature-x-variant feature-x
"$ORKTREE" ls
"$ORKTREE" rm feature-x-variant
"$ORKTREE" rm feature-x
