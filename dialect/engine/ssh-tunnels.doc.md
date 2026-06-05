# SSH Tunnels

When your database is behind a bastion or jump host, SELECT can open an **SSH tunnel** to reach it.

## Setup

In the connection form, switch the connection mode to **DSN + SSH tunnel**. This reveals the SSH configuration fields:

| Field           | Description                                              |
|-----------------|----------------------------------------------------------|
| **Host**        | Bastion/jump host hostname or IP (not the database host) |
| **Host key**    | Public key of the SSH server (proxified connections only) |
| **Port**        | SSH port, defaults to `22`                               |
| **User**        | System user on the SSH host                              |
| **Auth method** | `password` or `private_key`                              |
| **Password**    | SSH password (when auth method is password)              |
| **Private key** | Full PEM key content (when auth method is private_key)   |

> The **Host** field refers to the SSH jump host, not the database. The database host goes in the DSN.

## Host key verification

For **proxified connections**, the SSH host key is required. SELECT checks it on every connection to make sure you are reaching the right server. The server connects on your behalf, so it needs the key upfront.

Get it by running:

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
