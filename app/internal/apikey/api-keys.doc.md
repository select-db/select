# API Keys

API keys let automated clients authenticate to a workspace without a user signing in. A key is a workspace principal that carries **roles**, just like a user, so everything it does passes through the same [permission model](/docs/workspace/permissions/).

## Who can manage keys

Managing API keys requires the **Workspace API keys** permission, or workspace ownership. API keys themselves can never manage other keys, even if their roles would otherwise allow it.

## Creating a key

Open **Settings → API keys → New API key**. You choose:

- **Name**,  a label to recognise the key later.
- **Roles**,  one or more workspace roles assigned directly to the key. The key can only do what these roles allow; a key with no access to a database cannot query it. Keys are not [group](/docs/workspace/groups/) members, so give a key its roles directly.
- **Expiry**,  `Never`, or a fixed lifetime up to one year.

The secret is shown **once**, immediately after creation. Copy it then; it is stored only as a hash and cannot be retrieved again. If it is lost, rotate or revoke the key.

## Rotating a key

Rotating issues a **new** secret that inherits the old key's name, roles, and expiry. The old key keeps working for a 24‑hour grace window so a client can switch over without downtime, then stops automatically.

## Revoking a key

Revoking takes effect immediately: the next request made with that key is rejected. Revocation cannot be undone.

## Expiry

A key with no expiry never expires until revoked. When an expiry is set it is capped at one year. An expired key is rejected the same way a revoked one is.

> [!IMPORTANT]
> Treat a key's secret like a password. Scope each key to the minimum roles its task needs, and revoke keys that are no longer used.
