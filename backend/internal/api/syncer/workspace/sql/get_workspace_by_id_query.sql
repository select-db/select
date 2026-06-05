-- name: GetWorkspaceByID :one
SELECT id, name, git_remote_url, owner_id, updated_at, deleted_at FROM app.workspace WHERE id = $1;
