#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Build binary once
make -C "$SCRIPT_DIR/../.." build

export ORKTREE_BIN="$SCRIPT_DIR/../../build/orktree"

FILTER="${1:-test_*.sh}"
PASSED=0
FAILED=0
FAILURES=()

for test_file in "$SCRIPT_DIR"/$FILTER; do
  [[ -f "$test_file" ]] || continue
  name=$(basename "$test_file")
  echo "--- RUN  $name"
  if (bash "$test_file"); then
    echo "--- PASS $name"
    PASSED=$((PASSED + 1))
  else
    echo "--- FAIL $name"
    FAILED=$((FAILED + 1))
    FAILURES+=("$name")
  fi
done

echo ""
echo "=== SUMMARY: $PASSED passed, $FAILED failed ==="
if (( FAILED > 0 )); then
  for f in "${FAILURES[@]}"; do
    echo "  FAIL: $f"
  done
  exit 1
fi
