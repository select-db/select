# Audit Logs

The audit log is an append-only record of activity in a workspace: who did what,
to what, and when. Every security-relevant action (queries, permission and
membership changes, datasource changes) is written as an immutable event.

Read the log through the API at `GET /logs` (see the [API Reference](/api/)).
Reading it requires the `audit.read` workspace permission. The log is scoped to
the workspace, so it covers workspace activity only.

## The event model

Each event records that a **principal** performed an **action** (within a
**domain**) on a **target**, with an outcome (**status**), at a point in time.

| Field | Meaning |
|-------|---------|
| `domain` | The area the action belongs to: `query`, `iam`, `datasource`. |
| `action` | The action performed within that domain (see [Actions](#actions)). |
| `principal_type` / `principal_id` | Who acted: a `user` or an `api_key`, and its id. |
| `target_type` / `target_id` / `target_label` | What was acted on: a `permission`, `role`, `user`, or `datasource`. |
| `status` | Outcome: `success`, `error`, `failure`, `denied`. |
| `occurred_at` | When the action happened (event time). |
| `recorded_at` | When the event was stored; may lag `occurred_at`. |
| `payload` | Domain-specific details. |
| `client_ip` | The client's IP address. |

The full identity of an event is `domain.action`. Single-subject domains
(`query`, `datasource`) use a bare verb; `iam` uses a dotted `object.action`
hierarchy (the same shape as Okta's System Log): lifecycle changes are
`<object>.lifecycle.create` / `update` / `delete`, memberships are
`<container>.user_membership.add` / `remove`, and role grants are
`<subject>.role.grant` / `revoke`.

## Actions

The action vocabulary is a closed set: new actions are added over time, but
existing ones are never renamed, so you can rely on these values in filters.

### query

| Action | Meaning |
|--------|---------|
| `executed` | A statement ran, or was blocked. A blocked statement is `executed` with status `denied`. |

### iam

| Action | Meaning |
|--------|---------|
| `permission.lifecycle.create` / `update` / `delete` | A permission rule was created / updated / removed. |
| `role.lifecycle.create` / `update` / `delete` | A role was created / updated / removed. |
| `user.role.grant` / `user.role.revoke` | A role was granted to / revoked from a user. |
| `workspace.user_membership.add` / `workspace.user_membership.remove` | A user was added to / removed from the workspace. |
| `group.lifecycle.create` / `update` / `delete` | A group was created / updated / removed. |
| `group.user_membership.add` / `group.user_membership.remove` | A user was added to / removed from a group. |
| `group.role.grant` / `group.role.revoke` | A role was granted to / revoked from a group. |
| `workspace.lifecycle.create` / `delete` | The workspace was created / removed. |
| `api_key.lifecycle.create` / `rotate` / `revoke` / `api_key.role.set` | An API key was created / rotated / revoked / had its roles set. |

### datasource

| Action | Meaning |
|--------|---------|
| `created` / `updated` | A datasource was created / its connection config was changed. |
| `deleted` | A datasource was removed. |

## Querying the log

`GET /logs` accepts an OData `$filter` (see the [API Reference](/api/) for the
full grammar and the filterable fields). A few examples:

```
# permission changes that were denied
GET /logs?$filter=domain eq 'iam' and status eq 'denied'

# query executions blocked by permissions
GET /logs?$filter=domain eq 'query' and status eq 'denied'

# everything an API key did since the start of the year
GET /logs?$filter=principal_type eq 'api_key' and occurred_at ge 2026-01-01T00:00:00Z
```

## Retention

Events are partitioned by month and retained for one year by default.
