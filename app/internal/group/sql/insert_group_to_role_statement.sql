-- name: InsertGroupToRole :one
INSERT INTO group_to_role (id, group_id, role_id, workspace_id)
VALUES (:id, :group_id, :role_id, :workspace_id)
ON CONFLICT (group_id, role_id) DO NOTHING
RETURNING id, group_id, role_id, workspace_id, updated_at, deleted_at;
