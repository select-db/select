-- name: GetPermissionByID :one
SELECT id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect, updated_at, deleted_at
FROM app.permission
WHERE id = $1 AND workspace_id = $2;
