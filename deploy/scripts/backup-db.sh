#!/usr/bin/env bash
# dump PostgreSQL to a timestamped file.
# Usage: backup-db.sh <env> [label]
#   env:   staging | prod
#   label: optional prefix (e.g. "pre-deploy-1.2.3"), defaults to "manual"
# Output: /opt/selectdb/db/backups/<label>-<timestamp>.sql
set -euo pipefail

ENV=${1:?Usage: backup-db.sh <staging|prod> [label]}
LABEL=${2:-manual}
ENV_FILE="/opt/selectdb/env.${ENV}"
BACKUP_DIR="/opt/selectdb/db/backups"
KEEP_LAST=10

if [[ ! -f "${ENV_FILE}" ]]; then
    echo "[backup-db] ERROR: env file not found: ${ENV_FILE}" >&2
    exit 1
fi

# shellcheck disable=SC1090
source "${ENV_FILE}"

if [[ -z "${DB_DSN:-}" ]]; then
    echo "[backup-db] ERROR: DB_DSN is not set in ${ENV_FILE}" >&2
    exit 1
fi

mkdir -p "${BACKUP_DIR}"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
OUTPUT="${BACKUP_DIR}/${LABEL}-${TIMESTAMP}.sql"

echo "[backup-db] Backing up to ${OUTPUT} ..."
pg_dump "${DB_DSN}" > "${OUTPUT}"
echo "[backup-db] Backup complete: ${OUTPUT}"

# Prune old backups, keep the last N
BACKUP_COUNT=$(find "${BACKUP_DIR}" -maxdepth 1 -name "*.sql" | wc -l)
if [[ "${BACKUP_COUNT}" -gt "${KEEP_LAST}" ]]; then
    echo "[backup-db] Pruning old backups (keeping last ${KEEP_LAST}) ..."
    find "${BACKUP_DIR}" -maxdepth 1 -name "*.sql" \
        | sort \
        | head -n "-${KEEP_LAST}" \
        | xargs rm -f
fi

echo "${OUTPUT}"