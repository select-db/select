-- name: InsertDefaultWorkspace :exec
INSERT INTO app.workspace (id, name, owner_id, updated_at)
VALUES ($1, 'My Workspace', $2, now());
