-- +goose Up
-- +goose StatementBegin
-- The single source of every @app.* smart tag apigen reads. It runs last, after
-- the schema is fully in place, so each COMMENT targets a column in its final
-- shape (no tagging a column an earlier-then-dropped migration removes). Table
-- tags declare the sync/API opt-ins; column tags carry prose docs, closed-enum
-- @app.values, the default-list @app.sort, and @app.api.readonly.

-- Enroll every synced table. All are uniform (single id PK, workspace_id tenant,
-- membership scoping); everything structural (PK, tenant, cursor, soft-delete,
-- patchable set) derives from the catalog. Only the opt-ins and write
-- authorization (mirroring authorizeCommit) are declared. workspace stays
-- hand-written (self-tenant + owner-only delete).
COMMENT ON TABLE app.role              IS '@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage';
COMMENT ON TABLE app.user_to_role      IS '@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage';
COMMENT ON TABLE app.permission        IS '@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage';
COMMENT ON TABLE app."group"           IS '@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage';
COMMENT ON TABLE app.user_to_group     IS '@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage';
COMMENT ON TABLE app.group_to_role     IS '@app.sync @app.api.list|get @app.api.create|update|delete requires groups.manage, roles.manage';
COMMENT ON TABLE app.workspace_to_user IS '@app.sync @app.api.list|get @app.api.create|update|delete requires roles.manage, users.manage';

-- audit.event is the append-only activity log: exposed over the API as a
-- read-only resource (list/get) named "log", gated behind audit.read, and
-- deliberately NOT synced (server-owned, never written by clients). The internal
-- content-address hash is hidden from the API.
COMMENT ON TABLE audit.event IS '@app.entity log @app.api.list|get requires audit.read';

-- Column docs: a concise description (prose) plus @app.values for closed enums,
-- so the generated API reference explains each field and its allowed values.
-- These also feed the app's column hover docs.
-- Vocabulary is deliberately consistent: a principal performs an action (within
-- a domain) on a target, with a status. The same noun is reused everywhere
-- (principal, action, target, domain, status), never a synonym.
-- occurred_at carries @app.sort desc: the log has no updated_at, so it lists by
-- event time, newest first.
COMMENT ON COLUMN audit.event.principal_hash IS '@app.hide';
COMMENT ON COLUMN audit.event.id             IS 'Unique identifier of the event.';
COMMENT ON COLUMN audit.event.occurred_at    IS 'When the event occurred (event time). @app.sort desc';
COMMENT ON COLUMN audit.event.recorded_at    IS 'When the event was recorded; may lag occurred_at.';
COMMENT ON COLUMN audit.event.domain         IS 'Domain of the action. @app.values [query, iam, datasource]';
COMMENT ON COLUMN audit.event.action         IS 'Action performed within its domain. See the [audit log reference](/workspace/audit-logs/) for the full list of actions.';
COMMENT ON COLUMN audit.event.principal_id   IS 'Identifier of the principal.';
COMMENT ON COLUMN audit.event.principal_type IS 'Type of the principal that performed the action. @app.values [user, api_key]';
COMMENT ON COLUMN audit.event.target_id      IS 'Identifier of the target.';
COMMENT ON COLUMN audit.event.target_type    IS 'Type of the target the action was performed on. @app.values [permission, role, user, datasource]';
COMMENT ON COLUMN audit.event.target_label   IS 'Label of the target at the time of the event.';
COMMENT ON COLUMN audit.event.status         IS 'Outcome of the action. @app.values [success, error, failure, denied]';
COMMENT ON COLUMN audit.event.payload        IS 'Domain-specific details of the event.';
COMMENT ON COLUMN audit.event.client_ip      IS 'IP address of the client.';

-- A permission is a rule; consistent phrasing: "<scope> the rule applies to".
COMMENT ON COLUMN app.permission.action         IS 'SQL action the rule applies to. @app.values [select, insert, update, delete, ddl, see, manage]';
COMMENT ON COLUMN app.permission.effect         IS 'Whether the rule allows or denies the action. @app.values [allow, deny]';
COMMENT ON COLUMN app.permission.db_instance_id IS 'Database instance the rule applies to; null = any.';
COMMENT ON COLUMN app.permission.schema_name    IS 'Schema the rule applies to; null = any.';
COMMENT ON COLUMN app.permission.table_name     IS 'Table the rule applies to; null = any.';
COMMENT ON COLUMN app.permission.column_name    IS 'Column the rule applies to; null = any.';

-- role and group default their list sort to name, a friendlier listing than
-- updated_at.
COMMENT ON COLUMN app.role.name    IS 'Name of the role. @app.sort';
COMMENT ON COLUMN app."group".name IS 'Name of the group. @app.sort';

-- source/external_id are provenance owned by the (future) SCIM path, never the
-- client: @app.api.readonly keeps them readable and filterable but out of the
-- write body, so a client can't forge a row's origin or hijack an IdP object
-- mapping. The syncer still writes them (see IsWritable vs Patchable).
COMMENT ON COLUMN app."group".source       IS 'Origin of the group (e.g. local or an external provider). @app.api.readonly';
COMMENT ON COLUMN app."group".external_id  IS 'Identifier of the group in the external provider, when not local. @app.api.readonly';
COMMENT ON COLUMN app.user_to_group.source IS 'Origin of the membership (e.g. local or an external provider). @app.api.readonly';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
COMMENT ON TABLE app.role              IS NULL;
COMMENT ON TABLE app.user_to_role      IS NULL;
COMMENT ON TABLE app.permission        IS NULL;
COMMENT ON TABLE app."group"           IS NULL;
COMMENT ON TABLE app.user_to_group     IS NULL;
COMMENT ON TABLE app.group_to_role     IS NULL;
COMMENT ON TABLE app.workspace_to_user IS NULL;
COMMENT ON TABLE audit.event IS NULL;
COMMENT ON COLUMN audit.event.principal_hash IS NULL;
COMMENT ON COLUMN audit.event.id             IS NULL;
COMMENT ON COLUMN audit.event.occurred_at    IS NULL;
COMMENT ON COLUMN audit.event.recorded_at    IS NULL;
COMMENT ON COLUMN audit.event.domain         IS NULL;
COMMENT ON COLUMN audit.event.action         IS NULL;
COMMENT ON COLUMN audit.event.principal_id   IS NULL;
COMMENT ON COLUMN audit.event.principal_type IS NULL;
COMMENT ON COLUMN audit.event.target_id      IS NULL;
COMMENT ON COLUMN audit.event.target_type    IS NULL;
COMMENT ON COLUMN audit.event.target_label   IS NULL;
COMMENT ON COLUMN audit.event.status         IS NULL;
COMMENT ON COLUMN audit.event.payload        IS NULL;
COMMENT ON COLUMN audit.event.client_ip      IS NULL;
COMMENT ON COLUMN app.permission.action         IS NULL;
COMMENT ON COLUMN app.permission.effect         IS NULL;
COMMENT ON COLUMN app.permission.db_instance_id IS NULL;
COMMENT ON COLUMN app.permission.schema_name    IS NULL;
COMMENT ON COLUMN app.permission.table_name     IS NULL;
COMMENT ON COLUMN app.permission.column_name    IS NULL;
COMMENT ON COLUMN app.role.name    IS NULL;
COMMENT ON COLUMN app."group".name IS NULL;
COMMENT ON COLUMN app."group".source       IS NULL;
COMMENT ON COLUMN app."group".external_id  IS NULL;
COMMENT ON COLUMN app.user_to_group.source IS NULL;
-- +goose StatementEnd
