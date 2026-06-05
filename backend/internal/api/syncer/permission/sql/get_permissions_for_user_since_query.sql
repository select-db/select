-- name: GetPermissionsForUserSince :many
SELECT p.id, p.role_id, p.workspace_id, p.db_instance_id, p.schema_name, p.table_name, p.column_name, p.action, p.effect, p.updated_at, p.deleted_at
FROM app.permission p
INNER JOIN app.user_to_role ur ON ur.role_id = p.role_id
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = ur.workspace_id AND wtu.user_id = $1
WHERE p.updated_at > $2;
