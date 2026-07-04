-- name: GetUserGroupRolesWithNames :many
-- Roles a user holds indirectly, via group membership: user_to_group -> group_to_role -> role.
-- Unioned with GetUserRolesWithNames (direct roles) to form the effective role set baked
-- into the access token.
SELECT DISTINCT r.id, r.name, r.workspace_id
FROM app.role r
JOIN app.group_to_role gr ON gr.role_id = r.id AND gr.deleted_at IS NULL
JOIN app.user_to_group ug ON ug.group_id = gr.group_id AND ug.deleted_at IS NULL
WHERE ug.user_id = $1 AND r.deleted_at IS NULL;
