-- name: GetWorkspaceOwnerID :one
SELECT owner_id FROM workspace WHERE id = :id;
