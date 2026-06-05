#!/usr/bin/env bash
# resolve-pubkeys.sh — read the minisign public key for build-time embedding.
# Sourced by CI build jobs and scripts/build.sh. Sets PUBKEY.
# Fails if the public key file is missing.

set -euo pipefail

PUBKEY=""

PUBKEY_PATH="${1:-minisign.pub}"

if [[ ! -f "${PUBKEY_PATH}" ]]; then
    echo "ERROR: ${PUBKEY_PATH} is not committed." >&2
    echo "Generate a keypair locally and commit the public key before building:" >&2
    echo "  minisign -G -p ${PUBKEY_PATH} -s \$HOME/.minisign/select-db.sec" >&2
    exit 1
fi

PUBKEY=$(tail -n1 "${PUBKEY_PATH}")
