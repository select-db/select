-- name: GetRoleIDsByUserID :many
SELECT role_id FROM app.user_to_role
WHERE user_id = $1 AND deleted_at IS NULL;
