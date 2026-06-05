-- name: ListPermissionsByRole :many
SELECT id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect
FROM permission
WHERE role_id = :role_id
ORDER BY db_instance_id, schema_name, table_name, column_name, action;
