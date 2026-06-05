-- name: InsertWorkspaceIfNotExists :exec
INSERT INTO app.workspace (id, name, updated_at) 
VALUES ($1, 'Workspace', now())
ON CONFLICT (id) DO NOTHING;
