-- name: GetUserToGroupsForUserSince :many
SELECT ug.id, ug.user_id, ug.group_id, ug.workspace_id, ug.source, ug.updated_at, ug.deleted_at
FROM app.user_to_group ug
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = ug.workspace_id AND wtu.user_id = $1
WHERE ug.updated_at > $2;
