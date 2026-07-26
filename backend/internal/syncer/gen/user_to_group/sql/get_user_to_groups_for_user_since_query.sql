-- name: GetUserToGroupsForUserSince :many
SELECT r.id, r.user_id, r.group_id, r.workspace_id, r.source, r.updated_at, r.deleted_at
FROM app.user_to_group r
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = r.workspace_id AND wtu.user_id = $1
WHERE r.updated_at > $2;
