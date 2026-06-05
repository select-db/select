#!/usr/bin/env bash
# sign-and-deploy.sh — sign, notarize, and deploy a release.
#
# Runs on a maintainer's macOS laptop, not in CI. Picks up the release that CI
# built, codesigns and notarizes the macOS app bundles with Apple Developer ID,
# signs each desktop archive with minisign, uploads everything, and deploys the
# backend binary over SSH.
#
# This is the only step that touches the signing keys, Apple credentials, or
# SSH credentials. None of these should ever sit in CI.
#
# Usage:
#   ./deploy/sign-and-deploy.sh staging       # deploy staging
#   ./deploy/sign-and-deploy.sh v1.2.3        # deploy production
#
# Config (env or .env.release at repo root):
#   SSH_HOST                  prod box hostname
#   SSH_USER                  prod box user
#   STAGING_SSH_HOST          staging box hostname
#   STAGING_SSH_USER          staging box user
#   MINISIGN_SECRET_KEY       path to minisign .sec key (default: ~/.minisign/select-db.sec)
#   SSH_KEY                   optional path to SSH key file; default uses ssh-agent
#   APPLE_ID                  Apple ID email for notarytool
#   APPLE_TEAM_ID             Apple Developer Team ID (default: GF6M468A8B)
#   APPLE_APP_PASSWORD        app-specific password for notarytool
#   CODESIGN_IDENTITY         codesign identity (default: auto-detected from keychain)

set -euo pipefail

cd "$(dirname "$0")/.."

# macOS /sbin/sha256sum doesn't support -c, prefer shasum
if command -v shasum >/dev/null; then
    SHA256="shasum -a 256"
else
    SHA256="sha256sum"
fi

ARG="${1:-}"
if [[ -z "${ARG}" ]]; then
    echo "Usage: $0 <staging|vX.Y.Z>" >&2
    echo "  e.g. $0 staging" >&2
    echo "       $0 v1.2.3" >&2
    exit 1
fi

if [[ "${ARG}" == "staging" ]]; then
    ENV="staging"
    RELEASE_TAG="staging-latest"
    VERSION="staging"
elif [[ "${ARG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    ENV="prod"
    RELEASE_TAG="${ARG}"
    VERSION="${ARG}"
else
    echo "ERROR: argument must be 'staging' or vX.Y.Z (got: ${ARG})" >&2
    exit 1
fi

if [[ -f .env.release ]]; then
    set -a
    # shellcheck disable=SC1091
    source .env.release
    set +a
fi

if [[ "${ENV}" == "prod" ]]; then
    : "${SSH_HOST:?set SSH_HOST in .env.release or env}"
    : "${SSH_USER:?set SSH_USER in .env.release or env}"
    DEPLOY_HOST="${SSH_HOST}"
    DEPLOY_USER="${SSH_USER}"
else
    : "${STAGING_SSH_HOST:?set STAGING_SSH_HOST in .env.release or env}"
    : "${STAGING_SSH_USER:?set STAGING_SSH_USER in .env.release or env}"
    DEPLOY_HOST="${STAGING_SSH_HOST}"
    DEPLOY_USER="${STAGING_SSH_USER}"
fi

: "${APPLE_ID:?set APPLE_ID in .env.release or env}"
: "${APPLE_APP_PASSWORD:?set APPLE_APP_PASSWORD in .env.release or env}"
APPLE_TEAM_ID="${APPLE_TEAM_ID:-GF6M468A8B}"
MINISIGN_SECRET_KEY="${MINISIGN_SECRET_KEY:-${HOME}/.minisign/select-db.sec}"

# Auto-detect codesign identity if not set
if [[ -z "${CODESIGN_IDENTITY:-}" ]]; then
    CODESIGN_IDENTITY=$(security find-identity -v -p codesigning | grep "Developer ID Application" | head -1 | sed 's/.*"\(.*\)".*/\1/')
    if [[ -z "${CODESIGN_IDENTITY}" ]]; then
        echo "ERROR: no Developer ID Application certificate found in keychain." >&2
        echo "Install your certificate or set CODESIGN_IDENTITY in .env.release." >&2
        exit 1
    fi
    echo "    Using codesign identity: ${CODESIGN_IDENTITY}"
fi

if [[ ! -f minisign.pub ]]; then
    echo "ERROR: minisign.pub is not in the repo." >&2
    echo "Generate the keypair and commit the public key before tagging:" >&2
    echo "  minisign -G -p minisign.pub -s ${MINISIGN_SECRET_KEY}" >&2
    exit 1
fi

if [[ ! -f "${MINISIGN_SECRET_KEY}" ]]; then
    echo "ERROR: minisign secret key not found at ${MINISIGN_SECRET_KEY}" >&2
    echo "Set MINISIGN_SECRET_KEY to the correct path." >&2
    exit 1
fi

for tool in minisign gh ssh rsync codesign xcrun ditto; do
    if ! command -v "${tool}" >/dev/null; then
        echo "ERROR: required tool not found on PATH: ${tool}" >&2
        exit 1
    fi
done

SSH_OPTS=(-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)
if [[ -n "${SSH_KEY:-}" ]]; then
    if [[ ! -f "${SSH_KEY}" ]]; then
        echo "ERROR: SSH_KEY=${SSH_KEY} does not exist" >&2
        exit 1
    fi
    SSH_OPTS+=(-i "${SSH_KEY}")
fi
TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"

WORK="$(mktemp -d -t selectdb-release-XXXX)"
trap 'rm -rf "${WORK}"' EXIT

echo "==> [1/8] Fetching release ${RELEASE_TAG}"
gh release download "${RELEASE_TAG}" --dir "${WORK}" --clobber

shopt -s nullglob
MAC_ARCHIVES=("${WORK}"/selectDb-darwin-*.zip)

echo "==> [2/8] Verifying CI checksums"
if [[ ! -f "${WORK}/checksums.sha256" ]]; then
    echo "ERROR: checksums.sha256 missing from draft release" >&2
    exit 1
fi
( cd "${WORK}" && grep -v 'checksums.sha256' checksums.sha256 | ${SHA256} -c )

echo "==> [3/8] Codesigning macOS app bundles"
ENTITLEMENTS="app/build/darwin/entitlements.plist"
ANALYZER_ENTITLEMENTS="app/build/darwin/analyzer-entitlements.plist"
for ent in "${ENTITLEMENTS}" "${ANALYZER_ENTITLEMENTS}"; do
    if [[ ! -f "${ent}" ]]; then
        echo "ERROR: ${ent} not found" >&2
        exit 1
    fi
done
if [[ ${#MAC_ARCHIVES[@]} -eq 0 ]]; then
    echo "    No macOS archives found, skipping codesign"
else
    for archive in "${MAC_ARCHIVES[@]}"; do
        name=$(basename "${archive}")
        echo "    Codesigning ${name}"
        SIGN_WORK="$(mktemp -d -t selectdb-sign-XXXX)"
        ditto -xk "${archive}" "${SIGN_WORK}"
        APP=$(find "${SIGN_WORK}" -name "*.app" -maxdepth 1 -type d | head -1)
        if [[ -z "${APP}" ]]; then
            echo "ERROR: no .app bundle found in ${name}" >&2
            rm -rf "${SIGN_WORK}"
            exit 1
        fi
        # Sign inside-out. The analyzer is a PyInstaller sidecar that loads
        # unsigned dylibs at runtime, so it needs its own entitlements with
        # library validation disabled. --deep would re-sign it with the app's
        # entitlements and break it, so sign it explicitly first, then sign the
        # app without --deep so its signature is left intact.
        ANALYZER="${APP}/Contents/MacOS/analyzer"
        if [[ ! -f "${ANALYZER}" ]]; then
            echo "ERROR: analyzer sidecar missing from ${name}" >&2
            rm -rf "${SIGN_WORK}"
            exit 1
        fi
        codesign --force --options runtime \
            --sign "${CODESIGN_IDENTITY}" \
            --entitlements "${ANALYZER_ENTITLEMENTS}" \
            "${ANALYZER}"
        codesign --force --options runtime \
            --sign "${CODESIGN_IDENTITY}" \
            --entitlements "${ENTITLEMENTS}" \
            "${APP}"
        codesign --verify --verbose "${APP}"
        rm "${archive}"
        ditto -ck --keepParent "${APP}" "${archive}"
        rm -rf "${SIGN_WORK}"
    done
fi

echo "==> [4/8] Notarizing macOS archives"
for archive in "${MAC_ARCHIVES[@]}"; do
    name=$(basename "${archive}")
    echo "    Submitting ${name} for notarization"
    xcrun notarytool submit "${archive}" \
        --apple-id "${APPLE_ID}" \
        --team-id "${APPLE_TEAM_ID}" \
        --password "${APPLE_APP_PASSWORD}" \
        --wait
    STAPLE_WORK="$(mktemp -d -t selectdb-staple-XXXX)"
    ditto -xk "${archive}" "${STAPLE_WORK}"
    APP=$(find "${STAPLE_WORK}" -name "*.app" -maxdepth 1 -type d | head -1)
    xcrun stapler staple "${APP}"
    rm "${archive}"
    ditto -ck --keepParent "${APP}" "${archive}"
    rm -rf "${STAPLE_WORK}"
    echo "    ${name} notarized and stapled"
done

echo "==> [5/8] Signing desktop archives with minisign"
ARCHIVES=("${WORK}"/selectDb-*.zip)
if [[ ${#ARCHIVES[@]} -eq 0 ]]; then
    echo "ERROR: no selectDb-*.zip archives in draft release" >&2
    exit 1
fi
read -rsp "Minisign password: " MINISIGN_PASSWORD
echo
for archive in "${ARCHIVES[@]}"; do
    name=$(basename "${archive}")
    echo "    minisign -S ${name}"
    echo "${MINISIGN_PASSWORD}" | minisign -S \
        -s "${MINISIGN_SECRET_KEY}" \
        -t "version=${VERSION} archive=${name}" \
        -c "select-db ${VERSION}" \
        -m "${archive}"
done
unset MINISIGN_PASSWORD
( cd "${WORK}" && ${SHA256} -- *.zip *.minisig selectdb-backend > checksums.sha256 )

echo "==> [6/8] Uploading signatures and publishing release"
SIGS=("${WORK}"/*.minisig)
gh release upload "${RELEASE_TAG}" "${MAC_ARCHIVES[@]}" "${SIGS[@]}" "${WORK}/checksums.sha256" --clobber
if [[ "${ENV}" == "prod" ]]; then
    gh release edit "${RELEASE_TAG}" --draft=false
fi

echo "==> [7/8] Shipping backend binary to ${TARGET}"
BACKEND="${WORK}/selectdb-backend"
if [[ ! -f "${BACKEND}" ]]; then
    echo "ERROR: selectdb-backend missing from draft release" >&2
    exit 1
fi
chmod +x "${BACKEND}"
rsync -az -e "ssh ${SSH_OPTS[*]}" \
    "${BACKEND}" "${TARGET}:/opt/selectdb/backend/selectdb-backend.new"
rsync -az -e "ssh ${SSH_OPTS[*]}" \
    deploy/scripts/ "${TARGET}:/opt/selectdb/scripts/"

echo "==> [8/8] Running deploy-backend.sh on ${TARGET}"
ssh "${SSH_OPTS[@]}" "${TARGET}" \
    "chmod +x /opt/selectdb/scripts/*.sh /opt/selectdb/backend/selectdb-backend.new && /opt/selectdb/scripts/deploy-backend.sh ${ENV} ${VERSION}"

echo
echo "==> Done. ${VERSION} (${ENV}) is signed, published, and deployed."
