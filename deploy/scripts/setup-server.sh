#!/usr/bin/env bash
# setup-server.sh, one-time server provisioning.
# Usage: setup-server.sh <staging|prod>
# Run as root (or with sudo) on a fresh Debian/Ubuntu host.
set -euo pipefail

ENV=${1:?Usage: setup-server.sh <staging|prod>}

if [[ "${ENV}" != "staging" && "${ENV}" != "prod" ]]; then
    echo "ERROR: ENV must be 'staging' or 'prod'" >&2
    exit 1
fi

DOMAIN_MAP_staging="staging-api.select-db.com"
DOMAIN_MAP_prod="api.select-db.com"
DOMAIN_VAR="DOMAIN_MAP_${ENV}"
DOMAIN="${!DOMAIN_VAR}"

echo "==> [setup] ENV=${ENV} DOMAIN=${DOMAIN}"

# ── 1. System packages ─────────────────────────────────────────────────────────
echo "--> [1/7] Installing system packages ..."
apt-get update -q
apt-get install -y -q \
    curl \
    rsync \
    postgresql-client \
    nginx \
    certbot \
    python3-certbot-nginx \
    ufw \
    file

# ── 1b. Go ─────────────────────────────────────────────────────────────────────
GO_VERSION="1.24.2"
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
    echo "--> Installing Go ${GO_VERSION} ..."
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
        | tar -C /usr/local -xzf -
    echo "    Go ${GO_VERSION} installed at /usr/local/go"
else
    echo "    Go ${GO_VERSION} already installed, skipping."
fi
# Make go available system-wide
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# ── 2. OS user and directories ─────────────────────────────────────────────────
echo "--> [2/7] Creating user and directories ..."
if ! id selectdb &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin selectdb
    echo "    User 'selectdb' created"
fi

mkdir -p /opt/selectdb/{backend,db/backups,keys,src/backend,src/dev,scripts}
chown -R selectdb:selectdb /opt/selectdb
chmod 700 /opt/selectdb/keys
# scripts and src are written by the CI deploy user (root/sudo), not selectdb
chmod 755 /opt/selectdb/scripts /opt/selectdb/src

# ── 3. Env file ────────────────────────────────────────────────────────────────
echo "--> [3/7] Env file ..."
if [[ ! -f "/opt/selectdb/env.${ENV}" ]]; then
    cp "$(dirname "${BASH_SOURCE[0]}")/../env.example" "/opt/selectdb/env.${ENV}"
    chmod 600 "/opt/selectdb/env.${ENV}"
    echo "    Created /opt/selectdb/env.${ENV}, EDIT IT before deploying."
else
    echo "    /opt/selectdb/env.${ENV} already exists, skipping."
fi

# ── 4. RSA keys ────────────────────────────────────────────────────────────────
echo "--> [4/7] RSA key reminder ..."
if [[ ! -f "/opt/selectdb/keys/private.pem" ]]; then
    echo "    WARNING: /opt/selectdb/keys/private.pem not found."
    echo "    Copy your private.pem from the repo's backend/secrets/ to /opt/selectdb/keys/"
    echo "    Then set PRIVATE_KEY_PATH in /opt/selectdb/env.${ENV} (dev/local signing)."
    echo "    For prod, use an OVH KMS asymmetric key instead (set OVH_KMS_JWT_KEY_ID + OVH_KMS_*)."
fi

# ── 5. Systemd unit ────────────────────────────────────────────────────────────
echo "--> [5/7] Installing systemd unit ..."
UNIT_SRC="$(dirname "${BASH_SOURCE[0]}")/../systemd/selectdb-backend.service"
UNIT_DST="/etc/systemd/system/selectdb-backend.service"
install -m 644 "${UNIT_SRC}" "${UNIT_DST}"
# Point the EnvironmentFile to the correct env
sed -i "s|EnvironmentFile=.*|EnvironmentFile=/opt/selectdb/env.${ENV}|" "${UNIT_DST}"
systemctl daemon-reload
systemctl enable selectdb-backend
echo "    Systemd unit installed and enabled."

# ── 6. Firewall and Nginx ──────────────────────────────────────────────────────
echo "--> [6/7] Firewall and Nginx ..."
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw --force enable

NGINX_CONF="/etc/nginx/sites-available/selectdb"
install -m 644 "$(dirname "${BASH_SOURCE[0]}")/../nginx/selectdb.conf" "${NGINX_CONF}"
sed -i "s|SERVER_NAME_PLACEHOLDER|${DOMAIN}|g" "${NGINX_CONF}"
ln -sf "${NGINX_CONF}" /etc/nginx/sites-enabled/selectdb
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx

# Obtain SSL cert (comment out if you handle certs manually)
certbot --nginx -d "${DOMAIN}" --non-interactive --agree-tos -m admin@select-db.com

# ── 7. SSH authorized_keys for CI deploy user ─────────────────────────────────
echo "--> [7/7] SSH reminder ..."
echo "    Add your CI deploy public key to ~root/.ssh/authorized_keys (or the deploy user's)."
echo "    Then set these GitHub Actions secrets: STAGING_SSH_KEY, STAGING_HOST, STAGING_USER"

echo "==> [setup] Done. Next steps:"
echo "    1. Edit /opt/selectdb/env.${ENV} with real secrets"
echo "    2. Copy RSA keys to /opt/selectdb/keys/"
echo "    3. Push to the dev branch, CI will rsync source and deploy automatically."
