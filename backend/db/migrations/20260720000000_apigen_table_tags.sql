-- +goose Up
-- +goose StatementBegin
-- apigen: enroll every synced table. All are uniform (single id PK,
-- workspace_id tenant, membership scoping); everything structural (PK, tenant,
-- cursor, soft-delete, patchable set) derives from the catalog. Only the
-- opt-ins and write authorization (mirroring authorizeCommit) are declared.
-- workspace stays hand-written (self-tenant + owner-only delete).
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
COMMENT ON COLUMN audit.event.principal_hash     IS '@app.hide';
COMMENT ON COLUMN audit.event.id                 IS 'Unique event id.';
COMMENT ON COLUMN audit.event.occurred_at        IS 'When the event happened (event time).';
COMMENT ON COLUMN audit.event.recorded_at        IS 'When the event was persisted; may lag occurred_at.';
COMMENT ON COLUMN audit.event.domain             IS 'Event category. @app.values [query, auth, iam, datasource]';
COMMENT ON COLUMN audit.event.action             IS 'Action within the domain, e.g. executed, login, permission.upserted, role.assigned.';
COMMENT ON COLUMN audit.event.principal_id       IS 'Id of the actor (user or API key).';
COMMENT ON COLUMN audit.event.principal_type     IS 'Kind of actor. @app.values [user, api_key]';
COMMENT ON COLUMN audit.event.target_type        IS 'Kind of object acted on. @app.values [permission, role, user, datasource]';
COMMENT ON COLUMN audit.event.target_id          IS 'Id of the object acted on.';
COMMENT ON COLUMN audit.event.target_label       IS 'Human label of the target at event time.';
COMMENT ON COLUMN audit.event.status             IS 'Event outcome. @app.values [success, error, failure, denied]';
COMMENT ON COLUMN audit.event.payload            IS 'Domain-specific event details (JSON).';
COMMENT ON COLUMN audit.event.duration_ms        IS 'Duration in milliseconds (query events).';
COMMENT ON COLUMN audit.event.returned_row_count IS 'Rows returned (query events).';
COMMENT ON COLUMN audit.event.client_ip          IS 'Client IP address.';

COMMENT ON COLUMN app.permission.action         IS 'DB action governed. @app.values [select, insert, update, delete, ddl, see, manage]';
COMMENT ON COLUMN app.permission.effect         IS 'Whether the rule allows or denies. @app.values [allow, deny]';
COMMENT ON COLUMN app.permission.db_instance_id IS 'Target database instance; null = any.';
COMMENT ON COLUMN app.permission.schema_name    IS 'Target schema; null = any.';
COMMENT ON COLUMN app.permission.table_name     IS 'Target table; null = any.';
COMMENT ON COLUMN app.permission.column_name    IS 'Target column; null = any.';

COMMENT ON COLUMN app."group".source      IS 'Origin of the group, e.g. local or an external provider.';
COMMENT ON COLUMN app."group".external_id IS 'Identifier in the external provider, if sourced externally.';
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
COMMENT ON COLUMN audit.event.principal_hash     IS NULL;
COMMENT ON COLUMN audit.event.id                 IS NULL;
COMMENT ON COLUMN audit.event.occurred_at        IS NULL;
COMMENT ON COLUMN audit.event.recorded_at        IS NULL;
COMMENT ON COLUMN audit.event.domain             IS NULL;
COMMENT ON COLUMN audit.event.action             IS NULL;
COMMENT ON COLUMN audit.event.principal_id       IS NULL;
COMMENT ON COLUMN audit.event.principal_type     IS NULL;
COMMENT ON COLUMN audit.event.target_type        IS NULL;
COMMENT ON COLUMN audit.event.target_id          IS NULL;
COMMENT ON COLUMN audit.event.target_label       IS NULL;
COMMENT ON COLUMN audit.event.status             IS NULL;
COMMENT ON COLUMN audit.event.payload            IS NULL;
COMMENT ON COLUMN audit.event.duration_ms        IS NULL;
COMMENT ON COLUMN audit.event.returned_row_count IS NULL;
COMMENT ON COLUMN audit.event.client_ip          IS NULL;
COMMENT ON COLUMN app.permission.action         IS NULL;
COMMENT ON COLUMN app.permission.effect         IS NULL;
COMMENT ON COLUMN app.permission.db_instance_id IS NULL;
COMMENT ON COLUMN app.permission.schema_name    IS NULL;
COMMENT ON COLUMN app.permission.table_name     IS NULL;
COMMENT ON COLUMN app.permission.column_name    IS NULL;
COMMENT ON COLUMN app."group".source      IS NULL;
COMMENT ON COLUMN app."group".external_id IS NULL;
-- +goose StatementEnd
