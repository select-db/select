-- name: GetWorkspaceIDsByUserID :many
SELECT workspace_id FROM app.workspace_to_user WHERE user_id = $1;
