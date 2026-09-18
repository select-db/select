-- +goose Up
-- +goose StatementBegin
-- Auth events (login, token_refreshed, logout) are personal to a user, not a
-- workspace: a user's own security log, like GitHub's account audit, which an org
-- admin never sees. They carry no workspace, so workspace_id must be nullable.
-- Resource events (query/iam/datasource) stay workspace-scoped and always set it.
ALTER TABLE audit.event ALTER COLUMN workspace_id DROP NOT NULL;
ALTER TABLE audit.principal_snapshot ALTER COLUMN workspace_id DROP NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- User-scoped reads ("show me my auth history") filter by the actor, not the
-- workspace, so they can't lean on the workspace-leading indexes. This one serves
-- them; workspace-scoped reads keep using idx_event_ws_time et al.
CREATE INDEX IF NOT EXISTS idx_event_principal_id
    ON audit.event (principal_id, occurred_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS audit.idx_event_principal_id;
-- +goose StatementEnd

-- +goose StatementBegin
-- Restoring NOT NULL would fail if any auth event was written with a null
-- workspace; blank existing nulls to a sentinel is destructive, so the down
-- migration leaves the columns nullable. Re-adding the constraint is a manual,
-- data-aware step if ever needed.
-- +goose StatementEnd
