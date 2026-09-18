-- name: GetGroupToRolesForUserSince :many
SELECT r.id, r.group_id, r.role_id, r.workspace_id, r.updated_at, r.deleted_at
FROM app.group_to_role r
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = r.workspace_id AND wtu.user_id = $1
WHERE r.updated_at > $2;
