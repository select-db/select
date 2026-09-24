# datasource: managed SQLite, TODO

A managed database is a proxified datasource with `db_type: "sqlite"` whose
file SELECT hosts. Created from the app, the REST API or MCP; queried only
through the proxy, like any proxified connection. Target cost: one d2-4 node
plus Object Storage at ~$0.01/GB-month.

## Decisions

Settled; reopen with a reason, not a preference.

- **Engine**: plain SQLite files (`modernc.org/sqlite`), not `sqld`. libSQL is
  in maintenance mode and every access goes through the proxy, so its wire
  protocol buys nothing.
- **Access**: proxy only (app, REST, MCP). No direct client endpoint in v1; a
  libSQL endpoint can be added later without changing storage.
- **Model**: an `app.datasource` row plus `managed`, `node`, `size_bytes`,
  `state` (hot/cold), `last_used_at`. The DSN is assigned by the control plane;
  `upsert.go` keeps rejecting user-supplied sqlite.
- **Placement**: a separate node VM, the backend binary in node mode. Prod on
  a d2-4 in the proxy's region; staging co-located on the staging box. Every
  row records its `node` so moving or adding nodes is routing, not migration.
- **Storage**: Object Storage (Standard class) is the source of truth, the node
  disk is an LRU cache evicted under disk pressure. Wake is `restore`, the same
  path as disaster recovery. Never File Storage: NFS breaks SQLite locking.
- **Replication**: Litestream embedded as a pinned Go library, so the code that
  evicts and the code that replicates share a process and a per-db lock.
- **Proxy to node**: the existing engine HTTP transport
  (`dialect/engine/transport`), same routes the app uses against the proxy.
  Node listens on the private network only.
- **Proxy to node auth**: the existing JWT machinery (`auth/jwt.go`: KMS signer,
  RS256), not a new protocol. A separate `NodeClaims{db, ws}` with its own
  audience (`select-node`), so node tokens and user tokens never cross.
  60s expiry; the proxy caches one token per db for ~50s because every KMS
  `Sign` is a remote call. The node holds the public key only: no KMS access,
  cannot mint tokens. It rejects a request whose path db differs from `db`.
- **Isolation** (all required, none sufficient alone):
  - `SQLITE_LIMIT_ATTACHED=0` via `sqlite.Limit` on a pinned `*sql.Conn`: blocks
    `ATTACH` and `VACUUM INTO` (verified on v1.59.0).
  - `_defensive=1`, `trusted_schema(0)`; extensions are off in this build.
  - PRAGMA allowlist on the node (read-only introspection only). Needed because
    `PRAGMA temp_store_directory` is accepted and process-global, and the driver
    exposes no authorizer.
  - systemd sandbox: `ProtectSystem=strict`, `ReadWritePaths` on the data dir,
    `PrivateTmp`, egress to proxy and S3 only.
  - Node maintenance (fork, download, `VACUUM`) runs on a separate trusted conn.
- **Permissions**: create uses the existing upsert rule (owner, or `manage` on
  `*`). Each db gets a dedicated role (full access, `manage` included),
  created with it. `grant_to` is optional: the caller may always grant itself,
  others need the existing `users.manage` / `api-keys.manage`. Empty means
  deny-by-default, as for any proxified datasource.
- **Fork and download** require `manage` on the source: both hand over all the
  data, whatever the caller's column rules say.
- **Delete** is an immediate hard delete: local file, replica, and every role
  scoped only to that db. Multi-purpose roles lose their rules for it and stay.
- **Discovery**: the server is the source of truth. Settings lists managed dbs;
  "Add to workspace" shows the `db.config.json` in a modal to copy. The app
  never writes it; removing the folder removes a bookmark, not the db.
- **Quotas**, per workspace, from a new `workspace.plan` set by hand for now:

  |       | Total | Per db | Dbs | PITR  |
  | ----- | ----- | ------ | --- | ----- |
  | Solo  | 1 GB  | 500 MB | 10  | 1 day |
  | Teams | 20 GB | 5 GB   | 100 | 7 days |

  Per-db size is `PRAGMA max_page_count`; PITR is Litestream retention.
- **Names**: unique per workspace, same rules as folders.

## v1

### Control plane (backend)
- [ ] Migration: managed columns on `app.datasource`, `app.node`, `workspace.plan`
- [ ] Quota check on create and fork (count, total size)
- [ ] `POST /datasources/managed` create: row, dedicated role, `grant_to`, node pick
- [ ] `POST /datasources/{id}/fork`: `manage` on source, same create path
- [ ] `GET /datasources/{id}/download`: `manage`, streams a `VACUUM INTO` copy
- [ ] Delete: purge file and replica, drop db-only roles, strip rules elsewhere
- [ ] Route managed rows through the engine transport to their node
- [ ] `NodeClaims` (aud `select-node`, 60s) signed with the existing signer,
      cached per db for ~50s
- [ ] Audit events: reuse `datasource.lifecycle.*`

### Node
- [ ] Node mode in the backend binary: execute, schema, ping, dump routes
      served by `StreamLocal` against local files
- [ ] Token verification with the public key only; path db must match `db`
- [ ] Connection pinning per open db; limits and pragmas applied on open
- [ ] PRAGMA allowlist before execute
- [ ] Embedded Litestream per db, retention from the workspace plan
- [ ] LRU eviction on disk pressure: lock, checkpoint, sync, verify replica
      position, delete local file, mark `cold`
- [ ] Wake on first query: lock, restore, open, mark `hot`
- [ ] `size_bytes` and `last_used_at` reported to the control plane

### MCP
- [ ] `create_database`, `fork_database` (same code and checks as REST).
      No delete over MCP.

### App
- [ ] Create managed db from Settings
- [ ] Managed dbs in the Settings connections list, with size and state
- [ ] "Add to workspace" modal showing the `db.config.json`
- [ ] Delete with typed-name confirmation
- [ ] Download

### Ops (select-ops)
- [ ] Prod node: d2-4, vRack, systemd unit with sandbox
- [ ] Staging node: second unit on the staging box, own data dir
- [ ] Object Storage buckets (prod, staging) with versioning
- [ ] Alerts: node disk > 85%, replication lag > 1 min, any failed wake

### Tests
- [ ] Hostile SQL suite in CI: `ATTACH`, `VACUUM INTO`, `load_extension`,
      every non-allowlisted PRAGMA; fails the build if a driver bump changes
      any result
- [ ] Evict, wake, verify against MinIO in CI
- [ ] Node rejects expired, wrong-db, unsigned and user (wrong audience) tokens
- [ ] Benchmark Object Storage to d2-4 restore throughput before launch

### Docs
- [ ] `.doc.md` for managed databases: quotas, wake latency, delete is final

### Rollout
- [ ] Staging, then prod behind the sign-in allowlist, then everyone

## Later
- Compute budget and billing, globally: node time must count once it exists
- Abandoned db policy (`last_used_at` is recorded from v1)
- `issue_key` on create: an API key bound to the db role (Turso-style token)
- Upload of an existing `.db` (needs untrusted-file hardening)
- Size-aware eviction bias (evict small idle dbs first)
- Delete protection or a recovery window, as a Teams perk
- `delete_database` over MCP, limited to dbs whose role the key holds
- Authorizer API upstream in modernc, to replace the PRAGMA allowlist
- Direct libSQL endpoint; per-region proxy and node pairs
