#!/usr/bin/env bash
# Clear all data from the remote Postgres DB (tables unchanged) for retesting signup flow.
# Requires: DB_DSN (Postgres URL) and psql.
#
# Usage (from backend/):
#   ./scripts/reinit_db.sh
#   # or: DB_DSN='postgres://user:pass@localhost:5432/dbname?sslmode=disable' ./scripts/reinit_db.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Backend root: from backend/scripts/ go up one level
BACKEND_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load DB_DSN from .env if present and DB_DSN not already set
if [[ -z "${DB_DSN:-}" ]] && [[ -f "$BACKEND_ROOT/.env" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$BACKEND_ROOT/.env" 2>/dev/null || true
  set +a
fi

if [[ -z "${DB_DSN:-}" ]]; then
  echo "Error: DB_DSN is not set. Set it in the environment or in $BACKEND_ROOT/.env" >&2
  exit 1
fi

if ! command -v psql &>/dev/null; then
  echo "Error: psql is required (Postgres client)." >&2
  exit 1
fi

echo "Clearing data in remote Postgres DB..."

# TRUNCATE app.user CASCADE clears user and auth.refresh_token (FK to user)
psql "$DB_DSN" -v ON_ERROR_STOP=1 -c "TRUNCATE app.\"user\" CASCADE;"

echo "Done. Remote DB content cleared; you can retest the signup flow."
