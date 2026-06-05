#!/usr/bin/env bash
# deploy-backend.sh, deploy a pre-built binary.
# Usage: deploy-backend.sh <staging|prod> <version>
#
# Expects the binary already placed at /opt/selectdb/backend/selectdb-backend.new
# by CI (rsync'd before this script runs).
#
# Idempotent: safe to re-run.
set -euo pipefail

ENV=${1:?Usage: deploy-backend.sh <staging|prod> <version>}
VERSION="${2:?Usage: deploy-backend.sh <staging|prod> <version>}"
VERSION="${VERSION#v}"   # strip optional v prefix (e.g. v1.2.3 → 1.2.3)

if [[ "${ENV}" != "staging" && "${ENV}" != "prod" ]]; then
    echo "ERROR: ENV must be 'staging' or 'prod'" >&2
    exit 1
fi

ENV_FILE="/opt/selectdb/env.${ENV}"
DEPLOY_DIR="/opt/selectdb/backend"
BINARY="${DEPLOY_DIR}/selectdb-backend.new"
SERVICE="selectdb-backend"
PORT=8080
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> [deploy] ENV=${ENV} VERSION=${VERSION}"

# ── Prerequisites ─────────────────────────────────────────────────────────────
if [[ ! -f "${ENV_FILE}" ]]; then
    echo "ERROR: env file not found: ${ENV_FILE}" >&2
    exit 1
fi

# shellcheck disable=SC1090
set -a; source "${ENV_FILE}"; set +a

if [[ -z "${DB_DSN:-}" ]]; then
    echo "ERROR: DB_DSN is not set in ${ENV_FILE}" >&2
    exit 1
fi

if [[ ! -f "${BINARY}" ]]; then
    echo "ERROR: binary not found at ${BINARY}" >&2
    echo "       CI should rsync it before calling this script." >&2
    exit 1
fi

mkdir -p "${DEPLOY_DIR}"

# ── 1. Backup DB ─────────────────────────────────────────────────────────────
echo "--> [1/4] Backing up database ..."
BACKUP_FILE=$("${SCRIPTS_DIR}/backup-db.sh" "${ENV}" "pre-deploy-${VERSION}")
echo "    Backup: ${BACKUP_FILE}"

# ── 2. Run migrations ────────────────────────────────────────────────────────
echo "--> [2/4] Running migrations ..."
if ! "${BINARY}" migrate:up; then
    echo "ERROR: migrations failed, aborting" >&2
    rm -f "${BINARY}"
    exit 1
fi
echo "    Migrations OK"

# ── 3. Swap binary ───────────────────────────────────────────────────────────
echo "--> [3/4] Swapping binary ..."
if [[ -f "${DEPLOY_DIR}/selectdb-backend" ]]; then
    mv "${DEPLOY_DIR}/selectdb-backend" "${DEPLOY_DIR}/selectdb-backend.prev"
fi
mv "${BINARY}" "${DEPLOY_DIR}/selectdb-backend"

# ── 4. Restart + health check ────────────────────────────────────────────────
echo "--> [4/4] Restarting service ${SERVICE} ..."
sudo systemctl restart "${SERVICE}"

if ! "${SCRIPTS_DIR}/health-check.sh" "${PORT}"; then
    echo "ERROR: health check failed, rolling back" >&2
    if [[ -f "${DEPLOY_DIR}/selectdb-backend.prev" ]]; then
        mv "${DEPLOY_DIR}/selectdb-backend" "${DEPLOY_DIR}/selectdb-backend.failed"
        mv "${DEPLOY_DIR}/selectdb-backend.prev" "${DEPLOY_DIR}/selectdb-backend"
        sudo systemctl restart "${SERVICE}"
        echo "    Binary rolled back. DB was NOT restored automatically."
        echo "    If needed, run: rollback-backend.sh ${ENV}"
    fi
    exit 1
fi

echo "==> [deploy] SUCCESS, v${VERSION} is live on ${ENV}"
