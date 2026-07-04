-- +goose Up
-- +goose StatementBegin

-- A group is a named collection of users (identity), workspace-scoped like a role.
-- Groups attach to roles (group_to_role); permissions stay on roles, so there is a
-- single resolution path (user -> {direct roles} U {roles via groups} -> permissions).
-- "group" is a reserved word, hence the quoting throughout.
CREATE TABLE IF NOT EXISTS app."group" (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    -- Source of truth for this row: 'local' (app/syncer) today; an external IdP
    -- ('scim') once directory sync lands. Anchors IdP-vs-client write precedence
    -- so the sync engine's last-write-wins can defer to the IdP for synced rows.
    source       TEXT NOT NULL DEFAULT 'local',
    -- Stable IdP object id (SCIM externalId); NULL for locally-created groups.
    external_id  TEXT,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_group_updated_at ON app."group"(updated_at);
CREATE INDEX IF NOT EXISTS idx_group_workspace_id ON app."group"(workspace_id);
-- An IdP group maps to at most one local group per workspace.
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_workspace_external_id
    ON app."group"(workspace_id, external_id) WHERE external_id IS NOT NULL;

-- Membership: which users belong to a group. Source-tagged like the group itself
-- because SCIM owns membership while the app owns group definitions/mappings.
CREATE TABLE IF NOT EXISTS app.user_to_group (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES app."user"(id) ON DELETE CASCADE,
    group_id     UUID NOT NULL REFERENCES app."group"(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    source       TEXT NOT NULL DEFAULT 'local',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_to_group_updated_at ON app.user_to_group(updated_at);
-- Loaded to resolve a user's group-derived roles at token issuance.
CREATE INDEX IF NOT EXISTS idx_user_to_group_user_workspace ON app.user_to_group(user_id, workspace_id);
-- Loaded to fan out token revocation to every member when a group's roles change.
CREATE INDEX IF NOT EXISTS idx_user_to_group_group_id ON app.user_to_group(group_id);

-- Which roles a group grants. A member's effective roles are the union of their
-- direct user_to_role and the roles reachable through this table.
CREATE TABLE IF NOT EXISTS app.group_to_role (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id     UUID NOT NULL REFERENCES app."group"(id) ON DELETE CASCADE,
    role_id      UUID NOT NULL REFERENCES app.role(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_group_to_role_updated_at ON app.group_to_role(updated_at);
CREATE INDEX IF NOT EXISTS idx_group_to_role_group_id ON app.group_to_role(group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.group_to_role;
DROP TABLE IF EXISTS app.user_to_group;
DROP TABLE IF EXISTS app."group" CASCADE;
-- +goose StatementEnd
