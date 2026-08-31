-- name: UpdateWorkspaceLogo :exec
UPDATE app.workspace
SET logo = $2,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
