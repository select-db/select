# datasource: managed SQLite, TODO

A managed database is a proxified datasource with `db_type: "sqlite"` whose
file SELECT hosts. Created from the app, the REST API or MCP; queried only
through the proxy, like any proxified connection. Target cost: one d2-4 cellar
plus Object Storage at ~$0.01/GB-month.

## Decisions

Settled; reopen with a reason, not a preference.

- **Engine**: plain SQLite files (`modernc.org/sqlite`), not `sqld`. libSQL is
  in maintenance mode and every access goes through the proxy, so its wire
  protocol buys nothing.
- **Access**: proxy only (app, REST, MCP). No direct client endpoint in v1; a
  libSQL endpoint can be added later without changing storage.
- **Model**: an `app.datasource` row plus `managed`, `cellar_id`, `size_bytes`,
  `state` (hot/cold), `last_used_at`. The DSN is assigned by the control plane;
  `upsert.go` keeps rejecting user-supplied sqlite.
- **Name**: the database-hosting mode of the backend is the *cellar*, in code
  (`internal/cellar`, `select-backend cellar`), data (`app.cellar`), ops
  (`select-cellar.service`) and talk. User docs say "managed database".
- **Placement**: a separate VM, the backend binary in cellar mode. Prod on
  a d2-4 in the proxy's region; staging co-located on the staging box. Every
  row records its `cellar_id` so moving or adding cellars is routing, not migration.
- **Storage**: Object Storage (Standard class) is the source of truth, the cellar
  disk is an LRU cache evicted under disk pressure. Wake is `restore`, the same
  path as disaster recovery. Never File Storage: NFS breaks SQLite locking.
- **Replication**: Litestream embedded as a pinned Go library, so the code that
  evicts and the code that replicates share a process and a per-db lock.
- **Proxy to cellar**: the existing engine HTTP transport
  (`dialect/engine/transport`), same routes the app uses against the proxy.
  Cellar listens on the private network only.
- **Proxy to cellar auth**: the existing JWT machinery (`auth/jwt.go`: KMS signer,
  RS256), not a new protocol. A separate `CellarClaims{db, ws}` with its own
  audience (`select-cellar`), so cellar tokens and user tokens never cross.
  60s expiry; the proxy caches one token per db for ~50s because every KMS
  `Sign` is a remote call. The cellar holds the public key only: no KMS access,
  cannot mint tokens. It rejects a request whose path db differs from `db`.
- **Isolation** (all required, none sufficient alone):
  - `SQLITE_LIMIT_ATTACHED=0` via `sqlite.Limit` on a pinned `*sql.Conn`: blocks
    `ATTACH` and `VACUUM INTO` (verified on v1.59.0).
  - `_defensive=1`, `trusted_schema(0)`; extensions are off in this build.
  - PRAGMA allowlist on the cellar (read-only introspection only). Needed because
    `PRAGMA temp_store_directory` is accepted and process-global, and the driver
    exposes no authorizer.
  - systemd sandbox: `ProtectSystem=strict`, `ReadWritePaths` on the data dir,
    `PrivateTmp`, egress to proxy and S3 only.
  - Cellar maintenance (fork, download, `VACUUM`) runs on a separate trusted conn.
- **Permissions**: create uses the existing upsert rule (owner, or `manage` on
  `*`). Each db gets a dedicated role (full access, `manage` included),
  created with it. `grant_to` is optional: the caller may always grant itself,
  others need the existing `users.manage` / `api-keys.manage`. Empty means
  deny-by-default, as for any proxified datasource.
- **Fork and download** require `manage` on the source: both hand over all the
  data, whatever the caller's column rules say.
- **Delete** is an immediate hard delete: the db stops serving at once, then
  the reconciler purges the local file, the replica, and every role scoped only
  to that db. Multi-purpose roles lose their rules for it and stay. Deleting a
  workspace marks all its managed dbs the same way.
- **Reconciler** (backend, every 10 min): asks the cellar for its inventory
  (local files and replica prefixes), checks each id in Postgres, and sends an
  explicit signed purge for rows `deleting`, dbs of deleted workspaces, and
  ids with no row older than 24h. Never "delete all but this live set", so an
  empty or failed query purges nothing. Capped at 50 purges per run; hitting
  the cap stops and alerts. The cellar never reads Postgres.
- **Bucket versioning** with a lifecycle rule expiring noncurrent versions and
  stale delete markers after 7 days: a wrong purge is recoverable for a week,
  then data is gone. User contract: deleted data leaves storage within 7 days.
- **Discovery**: the server is the source of truth. Settings lists managed dbs;
  "Add to workspace" shows the `db.config.json` in a modal to copy. The app
  never writes it; removing the folder removes a bookmark, not the db.
- **Quotas**, per workspace, from a new `workspace.plan` set by hand for now:

  |       | Total | Per db | Dbs | PITR  |
  | ----- | ----- | ------ | --- | ----- |
  | Solo  | 1 GB  | 500 MB | 10  | 1 day |
  | Teams | 20 GB | 1 GB   | 100 | 7 days |

  Per-db size is `PRAGMA max_page_count`; PITR is Litestream retention. The
  1 GB Teams cap keeps a worst wake near 30s on a d2-4; raise it with data,
  never lower it.
- **Point-in-time restore**: a fork with `at`, Turso-style. The cellar restores
  the replica at that timestamp into a new db; the source is never touched.
  In-place restore is later.
- **Names**: unique per workspace, same rules as folders.
- **Routes**: create is `POST /datasources` for every type, fork and download
  are explicit routes (`POST .../{id}/fork`, `GET .../{id}/download`), delete is
  the existing `DELETE /datasources/{id}`. No route is specific to managed dbs.
- **Errors**: the cellar classifies at the source into a closed set; the backend
  passes the code through and each surface maps it (REST status and
  `{code, message, ref}`, MCP `toolError`, app message). Unclassified is
  `internal`, so a new failure mode fails closed rather than leaking paths,
  bucket URLs or cellar addresses. One request id, minted by the backend, is
  logged on both sides and shown as `ref`.

  | Code                  | Message to the caller                  | HTTP |
  | --------------------- | -------------------------------------- | ---- |
  | `sql_error`           | SQLite's message about their SQL       | 400  |
  | `forbidden_statement` | not allowed on managed databases       | 400  |
  | `quota_exceeded`      | which limit was hit                    | 403  |
  | `timeout`             | the existing timeout message           | 408  |
  | `waking`              | database is waking up, `Retry-After: 5` | 503  |
  | `unavailable`         | managed databases unavailable          | 503  |
  | `internal`            | internal error, with `ref`             | 500  |

  Token and routing failures are `internal` to the caller and loud in logs:
  they mean a backend bug, never a user mistake.
- **Waking**: a query on a cold db waits for the restore up to 15s
  (configurable, below nginx `proxy_read_timeout`), then returns `waking`
  while the restore continues. Concurrent requests share one restore.
- **Dev**: `./dev.sh backend start` stays the only command. With no cellar address
  configured, the server also starts the cellar as a second listener on
  `localhost:8081` in the same process, still through the HTTP transport and a
  token signed with the dev key. Litestream replicates to `backend/.dev/replica`
  (its `file` replica), so no MinIO. A cellar that fails to start logs a warning
  and managed routes return 503; the rest of dev is unaffected.

## v1

### Prerequisite
- [ ] `mcp.asToolError`: an unknown error returns `err.Error()` to the caller.
      Map it to `internal` with a `ref` and log the detail instead.

### Control plane (backend)
- [ ] Migration: managed columns on `app.datasource`, `app.cellar`, `workspace.plan`
- [ ] Quota check on create and fork (count, total size)
- [ ] `POST /datasources`: create any datasource with a server-generated id,
      sharing the create path and validation of `PUT /datasources/{id}`.
      `db_type: sqlite` with no DSN is managed (quota, cellar pick, file,
      dedicated role, `grant_to`); sqlite with a DSN stays rejected. Returns
      the id and the `db.config.json` content. Unique names make a retried
      create a 409, not a duplicate.
- [ ] `PUT /datasources/{id}` on a managed row: rename only; no conversion
      between managed and unmanaged in either direction
- [ ] `POST /datasources/{id}/fork`: `manage` on source, same create path;
      optional `at` (within the plan's PITR window) restores from the replica
- [ ] `GET /datasources/{id}/download`: `manage`, streams a `VACUUM INTO` copy
- [ ] Delete: stop serving, mark `deleting`; the reconciler does the rest
- [ ] Workspace soft delete marks its managed dbs `deleting`
- [ ] Reconciler: inventory, per-id decision, signed purge, then drop the row,
      db-only roles and rules elsewhere; 24h orphan age, 50 per run cap + alert
- [ ] Route managed rows through the engine transport to their cellar
- [ ] `CellarClaims` (aud `select-cellar`, 60s) signed with the existing signer,
      cached per db for ~50s
- [ ] Audit events: reuse `datasource.lifecycle.*`
- [ ] Request id sent to the cellar; cellar error codes passed through to REST,
      MCP and the app

### Cellar
- [ ] Cellar mode in the backend binary: execute, schema, ping, dump routes
      served by `StreamLocal` against local files
- [ ] Token verification with the public key only; path db must match `db`
- [ ] `GET /cellar/inventory` and `DELETE /cellar/dbs/{id}` (file and replica
      prefix), behind the same token check
- [ ] Connection pinning per open db; limits and pragmas applied on open
- [ ] PRAGMA allowlist before execute
- [ ] Embedded Litestream per db, retention from the workspace plan
- [ ] LRU eviction on disk pressure: lock, checkpoint, sync, verify replica
      position, delete local file, mark `cold`
- [ ] Wake on first query: lock, restore, open, mark `hot`; single restore
      shared by concurrent callers, wait capped at 15s then `waking`
- [ ] Error classification into the closed code set; everything else `internal`
- [ ] `size_bytes` and `last_used_at` reported to the control plane

### MCP
- [ ] `create_database`, `fork_database` with `at` (same code and checks as REST).
      No delete over MCP.

### App
- [ ] Create managed db from Settings
- [ ] Managed dbs in the Settings connections list, with size and state
- [ ] "Add to workspace" modal showing the `db.config.json`
- [ ] Delete with typed-name confirmation
- [ ] Download
- [ ] "Waking database..." after ~1s on a slow first query; auto-retry on
      `waking`

### Dev
- [ ] In-process cellar listener when no cellar address is configured
- [ ] `file` replica under `backend/.dev/`, gitignored
- [ ] Cellar startup failure degrades to 503 on managed routes only
- [ ] `./dev.sh backend start --s3`: optional MinIO for S3-specific debugging

### Ops (select-ops)
- [ ] Prod cellar: d2-4, vRack, systemd unit with sandbox
- [ ] nginx `proxy_read_timeout` above the 15s wake cap
- [ ] Staging cellar: second unit on the staging box, own data dir
- [ ] Object Storage buckets (prod, staging) with versioning and a 7-day
      noncurrent-version expiry; verify OVH supports `NoncurrentVersionExpiration`
- [ ] Alerts: cellar disk > 85%, replication lag > 1 min, any failed wake,
      reconciler cap hit

### Tests
- [ ] Hostile SQL suite in CI: `ATTACH`, `VACUUM INTO`, `load_extension`,
      every non-allowlisted PRAGMA; fails the build if a driver bump changes
      any result
- [ ] Evict, wake, verify against MinIO in CI
- [ ] Reconciler: failed or empty Postgres query purges nothing; cap stops the run
- [ ] Cellar rejects expired, wrong-db, unsigned and user (wrong audience) tokens
- [ ] No response body on any surface contains a cellar path, bucket URL or
      cellar address, for every error code
- [ ] Benchmark Object Storage to d2-4 restore throughput before launch

### Docs
- [ ] `.doc.md` for managed databases: quotas, wake latency, delete is final,
      data leaves storage within 7 days

### Rollout
- [ ] Staging, then prod behind the sign-in allowlist, then everyone

## Later
- Compute budget and billing, globally: cellar time must count once it exists
- Abandoned db policy (`last_used_at` is recorded from v1)
- `issue_key` on create: an API key bound to the db role (Turso-style token)
- Upload of an existing `.db` (needs untrusted-file hardening)
- Size-aware eviction bias (evict small idle dbs first)
- Delete protection or a recovery window, as a Teams perk
- `delete_database` over MCP, limited to dbs whose role the key holds
- Authorizer API upstream in modernc, to replace the PRAGMA allowlist
- Direct libSQL endpoint; per-region proxy and cellar pairs
- In-place restore with an automatic `{name}_old_{ts}` backup fork (Neon-style),
  if changing ids on restore hurts
- Pin Teams dbs hot so large ones never wait on a wake
