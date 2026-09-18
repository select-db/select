-- name: GetPermissionsForUserSince :many
SELECT r.id, r.role_id, r.workspace_id, r.db_instance_id, r.schema_name, r.table_name, r.column_name, r.action, r.effect, r.updated_at, r.deleted_at
FROM app.permission r
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = r.workspace_id AND wtu.user_id = $1
WHERE r.updated_at > $2;
