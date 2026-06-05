-- name: InsertRole :one
INSERT INTO role (id, workspace_id, name)
VALUES (:id, :workspace_id, :name)
RETURNING id, workspace_id, name, updated_at, deleted_at;
