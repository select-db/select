-- name: ListUserToRoleIDsByUserAndWorkspace :many
SELECT id FROM user_to_role
WHERE user_id = :user_id AND workspace_id = :workspace_id;
