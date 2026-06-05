-- name: GetUserToRolesForUserSince :many
SELECT ur.id, ur.user_id, ur.role_id, ur.workspace_id, ur.updated_at, ur.deleted_at
FROM app.user_to_role ur
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = ur.workspace_id AND wtu.user_id = $1
WHERE ur.updated_at > $2;
