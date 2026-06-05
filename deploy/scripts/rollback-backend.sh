#!/usr/bin/env bash
# rollback-backend.sh, restore previous binary and DB backup.
# Usage: rollback-backend.sh <staging|prod>
#
# What it does:
#   1. Restores the latest pre-deploy backup from /opt/selectdb/db/backups/
#   2. Swaps selectdb-backend.prev back to selectdb-backend
#   3. Restarts the service and runs a health check
#
# NOTE: migrate:down is intentionally NOT used here.
# In staging/prod, goose blocks 'down' outside dev. Rollback = restore backup.
set -euo pipefail

ENV=${1:?Usage: rollback-backend.sh <staging|prod>}

if [[ "${ENV}" != "staging" && "${ENV}" != "prod" ]]; then
    echo "ERROR: ENV must be 'staging' or 'prod'" >&2
    exit 1
fi

ENV_FILE="/opt/selectdb/env.${ENV}"
DEPLOY_DIR="/opt/selectdb/backend"
BACKUP_DIR="/opt/selectdb/db/backups"
SERVICE="selectdb-backend"
PORT=8080
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> [rollback] ENV=${ENV}"

# ── Prerequisites ──────────────────────────────────────────────────────────────
if [[ ! -f "${ENV_FILE}" ]]; then
    echo "ERROR: env file not found: ${ENV_FILE}" >&2
    exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

if [[ ! -f "${DEPLOY_DIR}/selectdb-backend.prev" ]]; then
    echo "ERROR: no previous binary found at ${DEPLOY_DIR}/selectdb-backend.prev" >&2
    exit 1
fi

# ── 1. Find latest pre-deploy backup ──────────────────────────────────────────
LATEST_BACKUP=$(find "${BACKUP_DIR}" -maxdepth 1 -name "pre-deploy-*.sql" \
    | sort \
    | tail -n 1)

if [[ -z "${LATEST_BACKUP}" ]]; then
    echo "ERROR: no pre-deploy backup found in ${BACKUP_DIR}" >&2
    echo "       You must restore the DB manually before running this script." >&2
    exit 1
fi

echo "--> [1/4] Restoring DB from ${LATEST_BACKUP} ..."
echo "    WARNING: this will DROP and recreate the public schema."
read -r -p "    Confirm? [y/N] " CONFIRM
if [[ "${CONFIRM}" != "y" && "${CONFIRM}" != "Y" ]]; then
    echo "    Aborted." >&2
    exit 1
fi

# Drop public schema and restore (safe for goose-managed schemas)
psql "${DB_DSN}" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
psql "${DB_DSN}" < "${LATEST_BACKUP}"
echo "    DB restored OK"

# ── 2. Swap binary ─────────────────────────────────────────────────────────────
echo "--> [2/4] Swapping to previous binary ..."
mv "${DEPLOY_DIR}/selectdb-backend" "${DEPLOY_DIR}/selectdb-backend.rolled-back"
mv "${DEPLOY_DIR}/selectdb-backend.prev" "${DEPLOY_DIR}/selectdb-backend"

# ── 3. Restart service ─────────────────────────────────────────────────────────
echo "--> [3/4] Restarting service ${SERVICE} ..."
systemctl restart "${SERVICE}"

# ── 4. Health check ────────────────────────────────────────────────────────────
echo "--> [4/4] Health check ..."
if ! "${SCRIPTS_DIR}/health-check.sh" "${PORT}"; then
    echo "ERROR: health check failed after rollback, manual intervention required" >&2
    exit 1
fi

echo "==> [rollback] SUCCESS, previous version is live on ${ENV}"
echo "    Restored backup: ${LATEST_BACKUP}"
