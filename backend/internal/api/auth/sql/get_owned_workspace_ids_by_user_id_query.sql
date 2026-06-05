-- name: GetOwnedWorkspaceIDsByUserID :many
SELECT id FROM app.workspace
WHERE owner_id = $1 AND deleted_at IS NULL;
