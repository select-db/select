-- +goose Up
-- +goose StatementBegin

-- Every statement that is not a plain data read or write now needs 'manage',
-- which is granted on the connection rather than per table. 'ddl' has no
-- meaning left: nothing maps to it, so a rule carrying it would sit in a role
-- looking like a grant and do nothing.
DELETE FROM app.permission WHERE action = 'ddl';

COMMENT ON COLUMN app.permission.action IS 'SQL action the rule applies to. @app.values [select, insert, update, delete, see, manage]';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
COMMENT ON COLUMN app.permission.action IS 'SQL action the rule applies to. @app.values [select, insert, update, delete, ddl, see, manage]';
-- +goose StatementEnd
