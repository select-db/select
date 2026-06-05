-- name: GetWorkspaceToUserForUserSince :many
SELECT wtu.id, wtu.workspace_id, wtu.user_id, wtu.updated_at, wtu.deleted_at
FROM app.workspace_to_user wtu
WHERE wtu.workspace_id IN (
    SELECT m.workspace_id FROM app.workspace_to_user m
    WHERE m.user_id = $1 AND m.deleted_at IS NULL
) AND wtu.updated_at > $2;
