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
-- +goose StatementEnd
