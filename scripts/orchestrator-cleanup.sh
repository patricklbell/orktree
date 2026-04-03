#!/usr/bin/env bash
# Stops and removes the ephemeral Docker container.  The orktree (branch +
# overlay data) is left intact so results can be inspected or merged.
set -euo pipefail

INPUT=$(cat)
TRANSCRIPT=$(printf '%s' "$INPUT" | jq -r '.transcript_path')

STATE_DIR="${HOME}/.orktree-fleet"
STATE_KEY=$(printf '%s' "$TRANSCRIPT" | sha1sum | cut -c1-16)
STATE_FILE="${STATE_DIR}/${STATE_KEY}.json"

# Nothing to clean — not a fleet session or already cleaned.
if [[ ! -f "$STATE_FILE" ]]; then
  exit 0
fi

CONTAINER_NAME=$(jq -r '.container_name' "$STATE_FILE")

# Stop and remove the container (ignore errors if already gone).
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

# Remove session state — the orktree persists for inspection.
rm -f "$STATE_FILE"

exit 0
