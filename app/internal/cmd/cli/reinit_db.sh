#!/usr/bin/env bash
# Clear all data from the app SQLite DB (tables unchanged) for retesting signup flow.
# APP_ENV is loaded from app/.env if not set (so use dev when app uses dev).
# Usage: ./cmd/cli/reinit_db.sh   (from app/backend)
# Or:    SELECTDB_DB_PATH=/path/to/db.sqlite ./cmd/cli/reinit_db.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_BACKEND_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
APP_ROOT="$(cd "$APP_BACKEND_DIR/.." && pwd)"

# Load APP_ENV from app/.env so script uses same env as the app (e.g. dev)
if [[ -z "${APP_ENV:-}" ]] && [[ -f "$APP_ROOT/.env" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$APP_ROOT/.env" 2>/dev/null || true
  set +a
fi

# DB path: match app backend GetAppDataDir()/db.sqlite, or override
if [[ -n "${SELECTDB_DB_PATH:-}" ]]; then
  DB_PATH="$SELECTDB_DB_PATH"
else
  APP_ENV="${APP_ENV:-default}"
  if [[ "$(uname -s)" == "Darwin" ]]; then
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/Library/Application Support}"
  else
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}"
  fi
  DB_PATH="$CONFIG_DIR/selectDb/$APP_ENV/localhost._.8080/db.sqlite"
fi

if ! command -v sqlite3 &>/dev/null; then
  echo "Error: sqlite3 is required (e.g. brew install sqlite)" >&2
  exit 1
fi

# If DB does not exist, create it from schema (so first-time reinit works)
SCHEMA_PATH="$APP_BACKEND_DIR/db/schema.sql"
if [[ ! -f "$DB_PATH" ]]; then
  if [[ ! -f "$SCHEMA_PATH" ]]; then
    echo "Error: DB file not found: $DB_PATH" >&2
    echo "Schema not found at $SCHEMA_PATH either. Run the app once to create the DB." >&2
    exit 1
  fi
  echo "Creating DB at $DB_PATH (from schema)"
  mkdir -p "$(dirname "$DB_PATH")"
  sqlite3 "$DB_PATH" < "$SCHEMA_PATH"
fi

echo "Clearing data in: $DB_PATH"

# Delete every workspace folder on disk (same app data dir as DB)
WORKSPACES_DIR="$(dirname "$DB_PATH")/workspaces"
if [[ -d "$WORKSPACES_DIR" ]]; then
  echo "Removing workspace folders: $WORKSPACES_DIR"
  rm -rf "$WORKSPACES_DIR"
fi

# Delete in FK-safe order (children first)
sqlite3 "$DB_PATH" "
  DELETE FROM mutation_commit;
  DELETE FROM workspace_to_user;
  DELETE FROM workspace;
  DELETE FROM history;
  DELETE FROM user;
"

echo "Done. DB content cleared and workspace folders removed; you can retest the signup flow."
