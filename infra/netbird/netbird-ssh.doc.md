# NetBird SSH Access

Draft plan and tooling for putting every machine we run behind NetBird Cloud, so
that TCP 22 can be closed on the public interface everywhere.

Internal operations doc. It is not listed in `web/docs/sidebar.txt` and does not
ship in the product documentation.

## What we run today

| Thing | Reachability |
|---|---|
| `selectdb-backend` hosts (staging, prod) | public IP, sshd on 22, OVH edge firewall |
| OVH managed Postgres | managed service, IP allowlist, no shell |
| OVH KMS | managed service, mTLS, no shell |
| Release signing laptop (`select-ops`) | no inbound at all |

Only the backend hosts have an sshd, so "every machine" means the backend fleet
plus anything we add later that we would otherwise expose on 22. The managed
services keep their own allowlists; NetBird does not change them.

## Goal

1. No machine accepts SSH from the internet.
2. Access is granted to a person, in one place, and revoked in the same place.
3. Losing NetBird costs us convenience, never the ability to recover a host.

## Design

**Membership.** Each host runs the NetBird agent and joins with a setup key
whose auto-groups put it in `srv-prod` or `srv-staging`. Nothing else decides a
host's group, so a host cannot end up in the wrong one by hand. Maintainer
laptops join through SSO and land in `admins`.

**Authorization.** NetBird ships an "allow all peers" default policy. It is
disabled by `bootstrap-account.sh`, because with it on, every peer can reach
every other peer and a compromised staging host has a path to prod. The
replacement is one policy: `admins` to `srv-prod` and `srv-staging`, TCP 22,
unidirectional. Servers get no peer-to-peer reachability at all.

**Enforcement on the host.** An nftables table accepts TCP 22 only from the
NetBird interface (`wt0`) or the NetBird CGNAT range (`100.64.0.0/10`), and
drops it everywhere else. The table's chain policy is `accept`, so it governs
port 22 and leaves the rest of the host's firewall alone.

The firewall does the enforcing rather than `sshd`'s `ListenAddress`, because
binding sshd to the NetBird address makes sshd fail to start when the agent has
not come up yet. The firewall rule is correct at every point in boot: before the
interface exists, the `iifname` match simply never hits and 22 is closed.

**Enforcement at the edge.** Once a host is verified reachable over NetBird, TCP
22 is dropped in the OVH network firewall too. The agent needs outbound only
(UDP 51820 to peers, TCP 443 to management, signal and relay), so nothing has to
be opened to replace it.

### Why not NetBird SSH yet

NetBird has an embedded SSH server (`netbird up --allow-server-ssh`) that
authenticates the user through OIDC and maps NetBird groups to local accounts,
which would let us delete `authorized_keys` entirely. It is the right end state
and this plan does not block it, but it is a second change: it moves both the
transport and the authentication at once, and its port 22 interception has to be
proven not to fight the running sshd. Stage A below closes the port with the
sshd and the keys we already trust. Stage B swaps the SSH server underneath,
on staging first, with the edge already closed and nothing to roll back at the
network layer.

## Files

| File | Runs where | What it does |
|---|---|---|
| `bootstrap-account.sh` | maintainer laptop | Creates groups, policies and setup keys through the NetBird API. Idempotent. |
| `enroll.sh` | target host, as root | Installs the agent and joins the network with a setup key. Idempotent. |
| `lockdown.sh` | target host, as root | Verifies NetBird reachability, then closes 22 with an automatic revert. |
| `files/select-ssh-guard.nft` | target host | The nftables table `lockdown.sh` installs. |
| `files/sshd-netbird.conf` | target host | sshd drop-in: no passwords, no root password login. |

## Rollout

Account setup runs once:

```sh
NETBIRD_TOKEN=<pat> ./bootstrap-account.sh
```

It prints the setup keys it created. A setup key is a credential: put it in the
OVH KMS secret store next to the other host secrets, not in this repo, and not
in a shell history file.

Every maintainer then installs the NetBird client, signs in with SSO, and is
added to the `admins` group in the dashboard.

Per host, in this order, with a working session open on the side:

```sh
# 1. join the network, port 22 still open
NB_SETUP_KEY=<key> NB_HOSTNAME=api-prod-1 ./enroll.sh

# 2. from a laptop on the network, prove SSH works over NetBird
ssh <user>@<netbird-ip>

# 3. close 22 to everything but NetBird, with a 15 minute auto-revert
./lockdown.sh apply

# 4. from the laptop, reconnect over NetBird and confirm
./lockdown.sh confirm
```

If step 4 does not happen, the revert timer restores the previous ruleset and
the host is reachable on 22 again. Only after `confirm` is the OVH edge rule for
22 removed, and that is the point of no easy return: from then on the console is
the fallback.

Do staging end to end first. Leave a week between staging and prod, long enough
for a NetBird agent update or a client login expiry to have happened at least
once.

## Break-glass

Losing every route to a host is the failure this design has to survive, so there
are three fallbacks and none of them depend on NetBird:

1. **OVH console.** KVM over the OVH manager reaches the host with no network at
   all. This is the primary fallback and it is why closing 22 is safe.
2. **The edge rule.** Re-adding TCP 22 in the OVH network firewall is an API
   call from any machine with the OVH credentials, and takes effect without
   touching the host.
3. **The revert timer.** During a lockdown window, `systemd-run` restores the
   saved ruleset unless `lockdown.sh confirm` has run.

Verify the console works on each host before step 3. An unverified console is
not a fallback.

## Failure modes

| Failure | Effect | What happens |
|---|---|---|
| NetBird management is down | No new sessions; existing WireGuard tunnels keep working | Peers hold their config; wait it out or use the console |
| Agent crashes on a host | That host is unreachable over SSH | `netbird` service restarts; console otherwise |
| Peer login expiry hits a server | Server drops off the network | Setup key peers are not subject to login expiry; keep it that way when changing account settings |
| Setup key leaks | Attacker can add a peer to `srv-*` | The policy grants `admins` to servers, not servers to anything, so a rogue peer in `srv-prod` reaches nothing. Revoke the key and delete the peer. |
| Laptop is stolen | Attacker holds a peer in `admins` | Delete the peer and disable the user in the IdP. This is the same blast radius as a stolen SSH key today, minus the key. |

## Config as code

`bootstrap-account.sh` is the source of truth for groups, policies and setup
keys. Changing them in the dashboard is fine for a one-off, but the next run
overwrites the policy body, so the durable change belongs in the script. The
script never deletes a group, a policy or a key it did not create.

## Open question

Deployment lives in the private `select-ops` repo (see the release job in
`.github/workflows/ci.yml`). These files are self-contained and have no
dependency on anything in this repository, so moving them there is a `git mv`.
Drafting them here keeps the review in the open; the decision on where they live
is the maintainer's.
