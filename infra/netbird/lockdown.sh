#!/usr/bin/env bash
# Close port 22 to everything but NetBird on this host, with a timer that undoes
# it unless the change is confirmed from a session that survived it.
#
# Run as root, after enroll.sh, with SSH over NetBird already proven to work.
#
# Usage:
#   ./lockdown.sh apply     install the guard, arm a 15 minute revert
#   ./lockdown.sh confirm   cancel the revert, keep the guard across reboots
#   ./lockdown.sh revert    remove the guard now
#   ./lockdown.sh status    what is installed and whether a revert is pending
#
# Environment:
#   NB_IFACE    NetBird interface, default wt0
#   NB_REVERT   revert delay, default 15min
#   NB_FORCE=1  skip the reachability check (console sessions have no peer)

set -euo pipefail

SELF=$(readlink -f "$0")
HERE=$(dirname "$SELF")
NB_IFACE=${NB_IFACE:-wt0}
NB_REVERT=${NB_REVERT:-15min}

GUARD_NFT=/etc/nftables.d/select-ssh-guard.nft
GUARD_UNIT=/etc/systemd/system/select-ssh-guard.service
SSHD_DROPIN=/etc/ssh/sshd_config.d/10-netbird.conf
REVERT_UNIT=select-ssh-unlock

if [[ $EUID -ne 0 ]]; then
  echo "error: run as root" >&2
  exit 1
fi

sshd_unit() {
  systemctl list-unit-files ssh.service >/dev/null 2>&1 && echo ssh || echo sshd
}

# Refuses to close the port until a live SSH session proves the NetBird path
# works. A peer address alone does not prove policies let anyone through.
check_reachable() {
  if [[ ${NB_FORCE:-} == 1 ]]; then
    return 0
  fi
  if ! ip link show "$NB_IFACE" >/dev/null 2>&1; then
    echo "error: no $NB_IFACE interface; run enroll.sh first" >&2
    exit 1
  fi
  if ! ss -tnH state established '( sport = :22 )' |
    grep -qE '100\.(6[4-9]|[7-9][0-9]|1[0-1][0-9]|12[0-7])\.'; then
    echo "error: no SSH session from the NetBird range is established." >&2
    echo "       Connect over NetBird first, or set NB_FORCE=1 if you are on the console." >&2
    exit 1
  fi
}

apply() {
  check_reachable

  mkdir -p "$(dirname "$GUARD_NFT")"
  sed "s/^define nb_iface = .*/define nb_iface = \"$NB_IFACE\"/" \
    "$HERE/files/select-ssh-guard.nft" >"$GUARD_NFT"
  nft -c -f "$GUARD_NFT"
  nft -f "$GUARD_NFT"

  install -m 0644 "$HERE/files/sshd-netbird.conf" "$SSHD_DROPIN"
  if ! sshd -t; then
    rm -f "$SSHD_DROPIN"
    echo "error: sshd rejected the drop-in; it has been removed" >&2
    exit 1
  fi
  systemctl reload "$(sshd_unit)"

  systemctl stop "$REVERT_UNIT.timer" >/dev/null 2>&1 || true
  systemd-run --unit="$REVERT_UNIT" --on-active="$NB_REVERT" "$SELF" revert >/dev/null

  echo "port 22 is now NetBird only. Reverting in $NB_REVERT unless confirmed."
  echo "next: open a new session over NetBird, then run: $SELF confirm"
}

confirm() {
  systemctl stop "$REVERT_UNIT.timer" >/dev/null 2>&1 || true
  install -m 0644 "$HERE/files/select-ssh-guard.service" "$GUARD_UNIT"
  systemctl daemon-reload
  systemctl enable select-ssh-guard.service >/dev/null
  echo "guard confirmed and enabled at boot."
  echo "next: drop TCP 22 for this host in the OVH network firewall."
}

revert() {
  systemctl disable --now select-ssh-guard.service >/dev/null 2>&1 || true
  nft delete table inet select_ssh_guard 2>/dev/null || true
  rm -f "$GUARD_NFT" "$GUARD_UNIT" "$SSHD_DROPIN"
  systemctl daemon-reload
  systemctl reload "$(sshd_unit)" 2>/dev/null || true
  echo "guard removed: port 22 is back to whatever the rest of the firewall says."
}

status() {
  if nft list table inet select_ssh_guard >/dev/null 2>&1; then
    echo "guard: loaded"
  else
    echo "guard: not loaded"
  fi
  systemctl is-enabled select-ssh-guard.service 2>/dev/null |
    sed 's/^/boot unit: /' || echo "boot unit: not installed"
  if systemctl is-active --quiet "$REVERT_UNIT.timer"; then
    echo "revert: pending, run confirm to cancel"
  else
    echo "revert: not armed"
  fi
  [[ -f $SSHD_DROPIN ]] && echo "sshd drop-in: installed" || echo "sshd drop-in: absent"
}

case ${1:-} in
apply) apply ;;
confirm) confirm ;;
revert) revert ;;
status) status ;;
*)
  echo "usage: $0 apply|confirm|revert|status" >&2
  exit 1
  ;;
esac
