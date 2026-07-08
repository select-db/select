-- name: GetGroupsForUserSince :many
SELECT g.id, g.workspace_id, g.name, g.source, g.external_id, g.updated_at, g.deleted_at
FROM app."group" g
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = g.workspace_id AND wtu.user_id = $1
WHERE g.updated_at > $2;
