-- name: GetWorkspaceOwnerID :one
SELECT owner_id FROM app.workspace WHERE id = $1;
