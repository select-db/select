# audit — TODO

Foundation merged: unified `audit.event` (partitioned by domain × month),
async + outbox lanes, principal snapshots, catalog/`Emit`, pg_partman/pg_cron
lifecycle. Live emit sites: `query.executed`, `iam.permission.lifecycle.*`.

## Wire remaining emit sites (vocabulary already declared in catalog.go)
- [ ] query: `denied`, `exported` (dump path)
- [ ] auth: `login`, `login_failed`, `token_refreshed`, `logout`
- [ ] iam: `role.lifecycle.create/update/delete`,
      `workspace.user_membership.add/remove`, `workspace.lifecycle.create/delete`,
      `api_key.lifecycle.create/rotate/revoke`
- [ ] datasource: `lifecycle.create/update`, `lifecycle.delete`

## Larger pieces
- [ ] Read API + `audit:read` authz + frontend (the consumption surface; nothing yet)
- [ ] OCSF/ECS field mapping in the read API (native SIEM ingest)
- [ ] Outbox atomicity: `LogOutbox` is a standalone INSERT after the mutation,
      not in its tx — thread a tx through `patch.Apply` (see TODO in logger.go)
- [ ] MCP execute path logging + `channel` (http|mcp) in payload (agent-access signal)
- [ ] System/platform events (key rotation, boot): blocked by `workspace_id NOT NULL` — needs a decision
- [ ] select-ops: enable pg_partman + pg_cron on the cluster, set `POSTGRES_AUDIT_CRON_DSN` (KMS secret → cluster's `defaultdb`)

## Notes
- `domain.action` is a frozen external contract — add, never rename/repurpose.
- Lanes: query/auth = async best-effort; iam/datasource = durable outbox.
