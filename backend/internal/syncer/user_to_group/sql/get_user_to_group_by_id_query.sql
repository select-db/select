-- name: GetUserToGroupByID :one
SELECT id, user_id, group_id, workspace_id, source, updated_at, deleted_at
FROM app.user_to_group
WHERE id = $1 AND workspace_id = $2;
