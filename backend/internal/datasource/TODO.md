# datasource: managed SQLite, TODO

A managed database is a SQLite database that SELECT hosts. Users create it
from the app, the REST API or MCP, and query it through the backend like any
proxified datasource. Target cost: one small VM plus Object Storage at about
$0.01 per GB-month.

## Words

- **backend**: the existing API server. Owns auth, permissions, plans, quotas
  and every row in Postgres.
- **cellar**: the same binary in cellar mode. Owns SQLite files and nothing
  else. Never reads Postgres, never knows a user.
- **hot / cold**: a db with a local file on the cellar / a db that lives only
  in the bucket.
- **wake**: restore a cold db from the bucket before running a query.
- **evict**: drop the local file of a hot db whose replica is complete.
- **reconciler**: the backend job that tells the cellar what to purge.

## How it works

```
app, REST, MCP --> backend --(signed token, private network)--> cellar --> bucket
                   auth, permissions,                           SQLite files,
                   plans, quotas, rows                          Litestream
```

1. The backend checks who is calling and what they may do, as for any
   proxified datasource.
2. It signs a 60s token for one db and forwards the request over the existing
   engine HTTP transport.
3. The cellar checks the token, wakes the db if cold, runs the statement under
   its limits and streams the result back.
4. Litestream streams every write to the bucket. The bucket is the truth, the
   cellar disk is a cache.

## Rules

Settled. Reopen with a reason, not a preference.

### Product
- A managed db is a datasource row with `db_type: "sqlite"`, no DSN, and a
  `cellar_id`; `cellar_id` and `state` are set together or not at all. A
  sqlite datasource with a DSN is still rejected: the server never opens a
  path a user gave it.
- Access goes through the backend only. No direct client endpoint in v1, so
  plain SQLite files (`modernc.org/sqlite`) rather than `sqld`.
- The server owns the list of managed dbs. The app shows them in Settings and
  "Add to workspace" shows a `db.config.json` to paste; removing the folder
  removes a bookmark, not the db.
- Opt-in: one `CELLAR` setting. Unset (default): managed dbs are off and the
  backend behaves as today. `local`: cellar in the same process (dev, small
  on-prem). A URL: remote cellar (prod). Off means every managed operation
  answers `disabled`; routes and MCP tools stay registered.

### Plans

|       | Total  | Per db | Dbs | Point-in-time window |
| ----- | ------ | ------ | --- | -------------------- |
| Solo  | 1 GB   | 250 MB | 10  | 1 day                |
| Teams | 20 GB  | 1 GB   | 100 | 7 days               |

- The plan is a new `workspace.plan`, set by hand until billing exists.
- The backend checks totals and counts on create and fork.
- The per-db cap and the window reach the cellar in the token (`max`,
  `pitr`). The cellar applies `max` as `max_page_count` on open; a db over a
  lowered cap keeps its data and stops growing. It keeps the last `pitr` seen
  per db and uses 7 days when it does not know, so history is never cut early.
- Raise limits with data, never lower them.

### Limits on every cellar statement
- Time: the workspace setting, capped at 60s, enforced by the cellar. Never
  read from the request.
- Concurrency: 10 statements in flight per workspace. More wait for a slot;
  the wait counts against the same 60s.
- Both are constants on the cellar, enforced by one `InFlight` middleware next
  to `RateLimit`, keyed by the token's `ws`. Query-seconds per workspace are
  recorded, not enforced.

### Routes
- `POST /datasources`: create any datasource type, server-generated id. Same
  validation as `PUT /datasources/{id}`. Returns the id and the
  `db.config.json` content. Names are free text, as for every datasource.
- `POST /datasources/{id}/fork`: optional `at` forks from a point in time into
  a new db. The source is never touched. The backend checks `at` is inside the
  plan's window.
- `GET /datasources/{id}/download`: the `.db` file.
- `PUT /datasources/{id}` on a managed db: rename only.
- `DELETE /datasources/{id}`: stops serving at once; the reconciler purges.
- MCP: `create_database` and `fork_database`, same code as REST. No delete.

### Permissions
- Create: the existing rule (owner, or `manage` on `*`).
- Each db gets a dedicated role with full access, created and deleted with it.
  `grant_to` is optional: the caller can always grant itself, anyone else
  needs `users.manage` or `api-keys.manage`. Empty means nobody, as for any
  proxified datasource.
- Fork and download need `manage` on the source: both hand over all the data.
- Deleting a db deletes roles scoped only to it and strips its rules from
  other roles.

### Cellar token
- `CellarClaims{db, ws, cel, max, pitr}`, audience `select-cellar`, 60s,
  signed with the existing JWT signer (`auth/jwt.go`). The backend caches one
  token per db for about 50s because each KMS sign is a remote call.
- The cellar holds only the public key. It rejects a token whose `db` differs
  from the path or whose `cel` is not itself. User tokens have another
  audience and never open a db.

### Storage
- Replicas live at `dbs/{db_id}/`, never under a cellar, so moving or
  recovering a db is a row update.
- Litestream is embedded as a pinned Go library: the code that evicts and the
  code that replicates share one process and one lock per db.
- Evict under disk pressure, least recently used first: lock, checkpoint,
  sync, check the replica is complete, delete the local file.
- Wake on first query: one restore shared by all waiting callers. Wait up to
  15s, then answer `waking` while the restore continues.
- Bucket: S3 with versioning and a 7-day expiry of old versions, so a wrong
  purge is recoverable for a week. Without S3 (dev, on-prem) Litestream writes
  to a directory; a startup preflight logs the mode, as for pg_partman, and
  warns when that directory shares the data disk.

### Isolation
All required, none sufficient alone. Checked on `modernc.org/sqlite` v1.59.0.
- `SQLITE_LIMIT_ATTACHED=0` on every user connection: blocks `ATTACH` and
  `VACUUM INTO`.
- `_defensive=1`, `trusted_schema(0)`; extensions are off in this build.
- PRAGMA allowlist, read-only introspection only. Needed because
  `temp_store_directory` is accepted and process-global, and the driver
  exposes no authorizer.
- systemd sandbox: data dir writable, nothing else; network to backend and
  bucket only; private network listener.
- Fork, download and maintenance use a separate trusted connection.
- No upload in v1.

### Cleanup
- Deleting a db or a workspace only marks rows `deleting`.
- The reconciler (backend, every 10 minutes, one backend at a time via a
  Postgres advisory lock) lists the cellar's dbs, checks each in Postgres, and
  sends one signed purge per db that is `deleting`, belongs to a deleted
  workspace, or has had no row for over 24h. It never says "keep these, delete
  the rest", so a failed query purges nothing. At most 50 purges per run;
  hitting that stops the run and alerts.

### Errors
The cellar maps every failure to one code; anything unmapped is `internal`, so
nothing leaks paths, bucket names or addresses. The backend creates a request
id, both sides log it, the caller sees it as `ref`.

| Code                  | Caller sees                              | HTTP |
| --------------------- | ---------------------------------------- | ---- |
| `sql_error`           | SQLite's message about their SQL         | 400  |
| `forbidden_statement` | not allowed on managed databases         | 400  |
| `quota_exceeded`      | which limit was hit                      | 403  |
| `timeout`             | the existing timeout message             | 408  |
| `waking`              | waking up, `Retry-After: 5`              | 503  |
| `unavailable`         | temporarily unavailable, retry           | 503  |
| `disabled`            | not enabled on this server, do not retry | 501  |
| `internal`            | internal error, with `ref`               | 500  |

### Scaling
- Any number of backends: wake dedup and limits live on the cellar.
- More cellars later: each row has a `cellar_id`; a move is mark `moving`,
  evict on the old cellar, flip `cellar_id`. The token's `cel` fences a stale
  route, so two cellars never write one replica.

### Environments
- Dev: `./dev.sh backend start` as today, with `CELLAR=local` and a directory
  replica in `backend/.dev/`. No MinIO. A cellar that fails to start only
  makes managed routes answer `unavailable`.
- Staging: a second systemd unit on the staging box, own data dir and bucket.
- Prod: a d2-4 VM on the private network, same region as the backend.

## v1 milestones

Each milestone ships behind `CELLAR` unset in prod and leaves `dev` green.
Numbers are order; items inside a milestone can run in parallel.

### 0. Groundwork
- [x] MCP errors: tool helpers return driver and network errors wrapped, and
      `asToolError` shows only tool errors and engine `ConfigError`s; anything
      else becomes `internal` with a `ref`.
- [x] `CELLAR` setting parsed at startup (`internal/cellar`); a bad value stops
      the server.
- [x] Migration: `workspace.plan`, and on `app.datasource`:
      `cellar_id`, `state`, `size_bytes`, `last_used_at`, and a check that a
      row with a cellar is SQLite with no DSN and a known state.

### 1. Cellar runs queries (local files, no bucket)
Needs 0.
- [ ] Cellar mode: execute, schema, ping, dump over the existing engine
      transport, `StreamLocal` against local files. `CELLAR=local` starts it
      in-process.
- [ ] `CellarClaims` signing with cache in the backend, verification in the
      cellar.
- [ ] Isolation: pragmas in the DSN, PRAGMA allowlist, and
      `sqlite.Limit(ATTACHED, 0)` on the `*sql.Conn` taken for each statement.
      The engine pools connections and the driver has no per-connection hook
      that can set limits, so per statement is the only fail-closed place.
- [ ] `InFlight` middleware, 60s cap, query-seconds recorded.
- [ ] Error codes and request id, end to end to REST and MCP. The request id
      lives in the context so `ref` matches the request log; MCP's mid-stream
      `collectSink` error goes through the same classification.
- [ ] Tests: hostile SQL suite (`ATTACH`, `VACUUM INTO`, `load_extension`,
      every non-allowlisted PRAGMA); token rejection (expired, wrong db, wrong
      cellar, unsigned, user token); no error body contains a path, bucket or
      address.

### 2. Create, fork, download, delete
Needs 1.
- [ ] Cellar: `PUT /cellar/dbs/{id}` with optional `{from, at}` (create,
      fork, point-in-time fork), `GET /cellar/dbs/{id}/download`,
      `GET /cellar/inventory`, `DELETE /cellar/dbs/{id}`.
- [ ] Backend: `POST /datasources`, fork, download, rename-only `PUT`,
      delete marking `deleting`; quota checks; dedicated role and `grant_to`.
- [ ] `disabled` on each of these entry points when `CELLAR` is unset.
- [ ] MCP `create_database`, `fork_database`.
- [ ] Audit: reuse `datasource.lifecycle.*`.

### 3. Bucket: replicate, evict, wake
Needs 1. Can run alongside 2. Start with the spikes.
- [ ] Spike: Litestream v0.5 as a library. Per-db replica with retention,
      read the replicated position, restore at a timestamp, `file` replica.
      One page of findings before building on it.
- [ ] Spike: OVH bucket supports `NoncurrentVersionExpiration`; restore speed
      from the bucket to a d2-4.
- [ ] Embedded Litestream per db at `dbs/{db_id}/`, window from `pitr`.
- [ ] Directory replica when no S3, with the startup preflight.
- [ ] LRU eviction on disk pressure; wake with shared restore and 15s wait.
- [ ] `size_bytes`, `state`, `last_used_at` reported back to the row.
- [ ] Point-in-time fork reads from the replica.
- [ ] Tests: evict, wake, verify, against MinIO and against a directory.

### 4. Cleanup
Needs 2 and 3.
- [ ] Reconciler with advisory lock, 24h orphan age, 50-per-run cap.
- [ ] Workspace delete marks its managed dbs `deleting`.
- [ ] Tests: a failed or empty Postgres query purges nothing; the cap stops
      the run.

### 5. App
Needs 2. Waking UI needs 3.
- [ ] Settings: create, list with size and state, delete with typed-name
      confirmation, download, "Add to workspace" modal with `db.config.json`.
- [ ] "Waking database..." after about 1s; retry on `waking`.
- [ ] `.doc.md`: plans, wake latency, delete is final, deleted data leaves
      storage within 7 days.

### 6. Ops and rollout
Needs 3 for staging, all for prod.
- [ ] Buckets (staging, prod) with versioning and 7-day old-version expiry.
- [ ] Staging cellar unit; prod d2-4 with systemd sandbox on the private
      network; nginx `proxy_read_timeout` above 15s.
- [ ] Alerts: disk over 85%, replication lag over 1 minute, failed wake,
      reconciler cap hit.
- [ ] Staging, then prod behind the sign-in allowlist, then everyone.

## Later
- Global compute budget and billing; cellar query-seconds must count.
- Policy for abandoned dbs (`last_used_at` is recorded from v1).
- `issue_key` on create: an API key bound to the db role.
- Upload of an existing `.db`, with untrusted-file hardening.
- In-place restore with an automatic backup fork, if changing ids hurts.
- Delete protection or a recovery window as a Teams perk.
- `delete_database` over MCP for dbs whose role the key holds.
- `managed_enabled` flag so the app can hide the create button.
- Second cellar: an `app.cellar` table for placement data (with a foreign key
  from `cellar_id`), `move`, dead-cellar runbook.
- Keep Teams dbs hot; evict small idle dbs first.
- Authorizer upstream in modernc, to replace the PRAGMA allowlist.
- Direct libSQL endpoint; per-region backend and cellar pairs.
- Workspace affinity for backends.
