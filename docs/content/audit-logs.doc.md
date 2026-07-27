# Audit Logs

The audit log is an append-only record of activity in a workspace: who did what,
to what, and when. Every security-relevant action (queries, sign-ins, permission
and membership changes, datasource changes) is written as an immutable event.

Read the log through the API at `GET /logs` (see the [API Reference](/api/)).
Reading it requires the `audit.read` workspace permission.

## The event model

Each event records that a **principal** performed an **action** (within a
**domain**) on a **target**, with an outcome (**status**), at a point in time.

| Field | Meaning |
|-------|---------|
| `domain` | The area the action belongs to: `query`, `auth`, `iam`, `datasource`. |
| `action` | The action performed within that domain (see [Actions](#actions)). |
| `principal_type` / `principal_id` | Who acted: a `user` or an `api_key`, and its id. |
| `target_type` / `target_id` / `target_label` | What was acted on: a `permission`, `role`, `user`, or `datasource`. |
| `status` | Outcome: `success`, `error`, `failure`, `denied`. |
| `occurred_at` | When the action happened (event time). |
| `recorded_at` | When the event was stored; may lag `occurred_at`. |
| `payload` | Domain-specific details. |
| `client_ip` | The client's IP address. |

The full identity of an event is `domain.action`. Single-subject domains
(`query`, `auth`, `datasource`) use a bare verb; `iam` spans several entities, so
its actions are `entity.verb`.

## Actions

The action vocabulary is a closed set: new actions are added over time, but
existing ones are never renamed, so you can rely on these values in filters.

### query

| Action | Meaning |
|--------|---------|
| `executed` | A statement ran, or was blocked. A blocked statement is `executed` with status `denied`. |

### auth

| Action | Meaning |
|--------|---------|
| `login` | A user signed in. |
| `login_failed` | A sign-in attempt failed. |
| `token_refreshed` | An access token was refreshed. |

Auth events are personal to a user and carry no workspace, so they appear in a
user's own security history rather than a workspace's log.

### iam

| Action | Meaning |
|--------|---------|
| `permission.upserted` / `permission.deleted` | A permission rule was created or updated / removed. |
| `role.upserted` / `role.deleted` | A role was created or updated / removed. |
| `role.assigned` / `role.unassigned` | A role was granted directly to a user / that grant was removed. |
| `member.added` / `member.removed` | A user was added to / removed from the workspace. |
| `group.upserted` / `group.deleted` | A group was created or updated / removed. |
| `group.member_added` / `group.member_removed` | A user was added to / removed from a group. |
| `group.role_attached` / `group.role_detached` | A role was attached to / detached from a group. |
| `workspace.created` / `workspace.deleted` | The workspace was created / removed. |
| `api_key.created` / `api_key.rotated` / `api_key.revoked` / `api_key.set_roles` | An API key was created / rotated / revoked / had its roles set. |

### datasource

| Action | Meaning |
|--------|---------|
| `upserted` | A datasource was created, or its connection config changed. |
| `deleted` | A datasource was removed. |

## Querying the log

`GET /logs` accepts an OData `$filter` (see the [API Reference](/api/) for the
full grammar and the filterable fields). A few examples:

```
# permission changes that were denied
GET /logs?$filter=domain eq 'iam' and status eq 'denied'

# failed sign-ins
GET /logs?$filter=action eq 'login_failed'

# everything an API key did since the start of the year
GET /logs?$filter=principal_type eq 'api_key' and occurred_at ge 2026-01-01T00:00:00Z
```

## Retention

Events are partitioned by month and retained for one year by default.
