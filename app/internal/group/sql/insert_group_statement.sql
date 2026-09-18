-- name: InsertGroup :one
INSERT INTO "group" (id, workspace_id, name)
VALUES (:id, :workspace_id, :name)
RETURNING id, workspace_id, name, source, external_id, updated_at, deleted_at;
