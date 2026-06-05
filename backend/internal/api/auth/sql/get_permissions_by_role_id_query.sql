-- name: GetPermissionsByRoleID :many
SELECT id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect, updated_at, deleted_at
FROM app.permission
WHERE role_id = $1 AND deleted_at IS NULL;
