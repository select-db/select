-- name: CountWorkspaceToUserByUserID :one
SELECT COUNT(*) FROM app.workspace_to_user WHERE user_id = $1;
