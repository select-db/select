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
| `domain` | The area the action belongs to: `query`, `auth`, `iam`, `datasource`. |
| `action` | The action performed within that domain (see [Actions](#actions)). |
| `principal_type` / `principal_id` | Who acted: a `user` or an `api_key`, and its id. |
| `target_type` / `target_id` / `target_label` | What was acted on: a `permission`, `role`, `user`, `group`, `workspace`, `api_key`, or `datasource`. |
| `status` | Outcome: `success`, `error`, `failure`, `denied`. |
| `occurred_at` | When the action happened (event time). |
| `recorded_at` | When the event was stored; may lag `occurred_at`. |
| `payload` | Domain-specific details. |
| `client_ip` | The client's IP address. |

The full identity of an event is `domain.action`. The `query` and `auth`
domains log activity with a bare verb (`executed`, `login`); `iam` and
`datasource` use a dotted `object.action` hierarchy (the same shape as Okta's
System Log): lifecycle
changes are `<object>.lifecycle.create` / `update` / `delete` (the object
prefix is dropped in `datasource`, where the domain is itself the object),
memberships are `<container>.user_membership.add` / `remove`, and role grants
are `<subject>.role.grant` / `revoke`.

## Actions

The action vocabulary is a closed set: new actions are added over time, but
existing ones are never renamed, so you can rely on these values in filters.
Each one below is the literal value of the `action` field, within the domain it
is listed under.

### query

- `executed`: a statement ran against a datasource. A statement blocked by
  permissions is `executed` with status `denied`, not an action of its own.

### auth

- `login`: a principal authenticated and a token was issued.
- `login_failed`: an authentication attempt was rejected (status `failure`).
- `token_refreshed`: an access token was reissued from a refresh token.

### iam

- `permission.lifecycle.create`: a permission rule was created.
- `permission.lifecycle.update`: a permission rule was updated.
- `permission.lifecycle.delete`: a permission rule was deleted.
- `role.lifecycle.create`: a role was created.
- `role.lifecycle.update`: a role was updated.
- `role.lifecycle.delete`: a role was deleted.
- `user.role.grant`: a role was granted directly to a user.
- `user.role.revoke`: a role granted directly to a user was revoked.
- `workspace.user_membership.add`: a user was added to the workspace.
- `workspace.user_membership.remove`: a user was removed from the workspace.
- `group.lifecycle.create`: a group was created.
- `group.lifecycle.update`: a group was updated, for example renamed.
- `group.lifecycle.delete`: a group was deleted.
- `group.user_membership.add`: a user was added to a group.
- `group.user_membership.remove`: a user was removed from a group.
- `group.role.grant`: a role was granted to a group, and so to every user in it.
- `group.role.revoke`: a role was revoked from a group.
- `workspace.lifecycle.create`: the workspace was created.
- `workspace.lifecycle.delete`: the workspace was deleted.
- `api_key.lifecycle.create`: an API key was created.
- `api_key.lifecycle.rotate`: an API key was rotated.
- `api_key.lifecycle.revoke`: an API key was revoked.
- `api_key.role.set`: an API key's bound roles were replaced.

### datasource

- `lifecycle.create`: a datasource was created.
- `lifecycle.update`: a datasource's connection config was changed.
- `lifecycle.delete`: a datasource was deleted.

## Querying the log

`GET /logs` accepts an OData `$filter` (see the [API Reference](/api/)) for the
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
