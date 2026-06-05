-- name: InsertUserToRole :one
INSERT INTO user_to_role (id, user_id, role_id, workspace_id)
VALUES (:id, :user_id, :role_id, :workspace_id)
ON CONFLICT (user_id, role_id) DO NOTHING
RETURNING id, user_id, role_id, workspace_id, updated_at, deleted_at;
