-- name: GetGroupByID :one
SELECT id, workspace_id, name, source, external_id, updated_at, deleted_at
FROM app."group"
WHERE id = $1;
