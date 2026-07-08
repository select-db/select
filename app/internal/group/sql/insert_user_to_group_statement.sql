-- name: InsertUserToGroup :one
INSERT INTO user_to_group (id, user_id, group_id, workspace_id)
VALUES (:id, :user_id, :group_id, :workspace_id)
ON CONFLICT (user_id, group_id) DO NOTHING
RETURNING id, user_id, group_id, workspace_id, source, updated_at, deleted_at;
