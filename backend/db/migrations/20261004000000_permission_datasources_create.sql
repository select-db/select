-- +goose Up
-- +goose StatementBegin

-- Adding a datasource used to check a workspace-level 'manage', a rule no role
-- screen could write. It checks workspace/datasources.create now, so a rule
-- that was written another way keeps the right it gave.
UPDATE app.permission
SET action = 'workspace/datasources.create'
WHERE datasource_id IS NULL AND action = 'manage';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE app.permission
SET action = 'manage'
WHERE datasource_id IS NULL AND action = 'workspace/datasources.create';
-- +goose StatementEnd
