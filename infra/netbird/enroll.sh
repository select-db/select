#!/usr/bin/env bash
# Install the NetBird agent on this host and join the network with a setup key.
# Idempotent: re-running on a connected host only reports its address.
#
# Run as root, on Debian or Ubuntu. Leaves the firewall untouched; closing
# port 22 is lockdown.sh, after this host is proven reachable over NetBird.
#
# Usage:
#   NB_SETUP_KEY=<key> NB_HOSTNAME=api-prod-1 ./enroll.sh
#   NB_SETUP_KEY_FILE=/run/secrets/netbird ./enroll.sh

set -euo pipefail

NB_HOSTNAME=${NB_HOSTNAME:-$(hostname -s)}
NB_TIMEOUT=${NB_TIMEOUT:-60}
KEYRING=/usr/share/keyrings/netbird-archive-keyring.gpg

if [[ $EUID -ne 0 ]]; then
  echo "error: run as root" >&2
  exit 1
fi

if [[ -n ${NB_SETUP_KEY_FILE:-} ]]; then
  NB_SETUP_KEY=$(<"$NB_SETUP_KEY_FILE")
fi

if [[ -z ${NB_SETUP_KEY:-} ]]; then
  echo "error: set NB_SETUP_KEY or NB_SETUP_KEY_FILE" >&2
  exit 1
fi

command -v jq >/dev/null || {
  echo "error: jq is required" >&2
  exit 1
}

install_agent() {
  command -v netbird >/dev/null && return 0
  command -v apt-get >/dev/null || {
    echo "error: no apt-get; install the agent by hand on this platform" >&2
    exit 1
  }
  echo "installing the netbird agent"
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl gnupg
  curl -fsSL https://pkgs.netbird.io/debian/public.key | gpg --dearmor --yes -o "$KEYRING"
  echo "deb [signed-by=$KEYRING] https://pkgs.netbird.io/debian stable main" \
    >/etc/apt/sources.list.d/netbird.list
  apt-get update -qq
  apt-get install -y -qq netbird
}

netbird_ip() {
  netbird status --json 2>/dev/null | jq -r '.netbirdIp // empty' | cut -d/ -f1
}

install_agent
systemctl enable --now netbird >/dev/null 2>&1 || netbird service install

if [[ -z $(netbird_ip) ]]; then
  netbird up --setup-key "$NB_SETUP_KEY" --hostname "$NB_HOSTNAME"
fi

# netbird up returns before the peer has an address, so poll rather than trust it.
for ((i = 0; i < NB_TIMEOUT; i++)); do
  ip=$(netbird_ip)
  if [[ -n $ip ]]; then
    break
  fi
  sleep 1
done

if [[ -z ${ip:-} ]]; then
  echo "error: no NetBird address after ${NB_TIMEOUT}s" >&2
  netbird status >&2
  exit 1
fi

echo "$NB_HOSTNAME is on the network at $ip"
echo "next: ssh to $ip from an admin peer, then run lockdown.sh apply"
