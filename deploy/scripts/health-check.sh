#!/usr/bin/env bash
# health-check.sh, waits for /health to return 200.
# Usage: health-check.sh [port] [max_retries] [sleep_secs]
#   Defaults: port=8080, max_retries=15, sleep_secs=2
# Exit 0 on success, 1 on timeout.
set -euo pipefail

PORT=${1:-8080}
MAX_RETRIES=${2:-15}
SLEEP=${3:-2}
URL="http://localhost:${PORT}/health"

echo "[health-check] Waiting for ${URL} ..."

for i in $(seq 1 "${MAX_RETRIES}"); do
    if curl -sf "${URL}" > /dev/null 2>&1; then
        echo "[health-check] OK (attempt ${i})"
        exit 0
    fi
    echo "[health-check] Attempt ${i}/${MAX_RETRIES}, not ready, retrying in ${SLEEP}s ..."
    sleep "${SLEEP}"
done

echo "[health-check] FAILED after ${MAX_RETRIES} attempts" >&2
exit 1
