# SSH Tunnels

When your database is behind a bastion or jump host, SELECT can open an **SSH tunnel** to reach it.

## Setup

In the connection form, switch the connection mode to **DSN + SSH tunnel**. This reveals the SSH configuration fields:

| Field           | Description                                              |
|-----------------|----------------------------------------------------------|
| **Host**        | Bastion/jump host hostname or IP (not the database host) |
| **Host key**    | Public key of the SSH server (auto-resolved locally; required for proxified) |
| **Port**        | SSH port, defaults to `22`                               |
| **User**        | System user on the SSH host                              |
| **Auth method** | `agent`, `key_file`, `password`, or `private_key`        |
| **Password**    | SSH password (when auth method is password)              |
| **Private key** | Full PEM key content (when auth method is private_key)   |
| **Key file**    | Path to a private key on disk (local `key_file` auth; only the path is stored) |

> The **Host** field refers to the SSH jump host, not the database. The database host goes in the DSN.

## Authentication methods

Local (non-proxified) connections run on the user's own machine, so SELECT can authenticate without storing any secret:

| Method        | Mode  | Stored | Notes                                                                 |
|---------------|-------|--------|----------------------------------------------------------------------|
| `agent`       | local | nothing | Default. Uses `SSH_AUTH_SOCK`; encrypted keys handled by the agent.  |
| `key_file`    | local | path only | Reads the key at connect time; prompts for a passphrase if encrypted (held in memory only). |
| `password`    | both  | value/`$var` | Prefer an `.env` `$variable` over a raw value.                  |
| `private_key` | both  | value/`$var` | Raw PEM (proxified) or an `.env` `$variable` (local).           |

`agent` and `key_file` are **desktop only**: they read the user's agent / a local file and are rejected when the outbound guard (`EnforceOutboundGuard`) is on, since the proxy server cannot reach the user's machine.

## Host key verification

A pinned host key lets SELECT verify, on every connection, that it is reaching the real bastion and not an impostor (a man-in-the-middle). When a host key is pinned, a mismatch aborts the connection. Whether a key is pinned depends on the mode:

**Local connections** pin automatically when possible: at connect time SELECT looks the host up in `~/.ssh/known_hosts` and pins that key, reusing the trust your own `ssh` client already established, so there is nothing to fill in. If the host is **not** in `known_hosts`, there is no key to pin, so the connection proceeds **unverified**. To get verification, pin the key first: either `ssh` to the host once so it lands in `known_hosts`, or set `host_key` in `db.config.json` directly.

**Proxified connections** always require a pinned key, they refuse to connect without one. The server connects on your behalf and has no access to your machine, so you must supply the key upfront. Get it by running:

```bash
ssh-keyscan bastion.example.com
```

The output looks like this (lines starting with `#` are comments, ignore them):

```
# bastion.example.com:22 SSH-2.0-OpenSSH_8.7
bastion.example.com ecdsa-sha2-nistp256 AAAAE2VjZHNh...
bastion.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5...
```

Paste any of the non-comment lines into the host key field. SELECT will lock the connection to that key type automatically.

## How it works

SELECT opens a **local port-forward** through the SSH host to the database. The DSN connects to the forwarded local port transparently.

Connections are **pooled and kept alive** to avoid re-establishing tunnels on every query. If the tunnel drops, SELECT detects it via keepalive and reconnects on the next query.
