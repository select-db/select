-- name: GetGroupToRolesForUserSince :many
SELECT gr.id, gr.group_id, gr.role_id, gr.workspace_id, gr.updated_at, gr.deleted_at
FROM app.group_to_role gr
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = gr.workspace_id AND wtu.user_id = $1
WHERE gr.updated_at > $2;
