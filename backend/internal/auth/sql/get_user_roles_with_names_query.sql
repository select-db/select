-- name: GetUserRolesWithNames :many
SELECT r.id, r.name
FROM app.role r
JOIN app.user_to_role utr ON utr.role_id = r.id
WHERE utr.user_id = $1 AND utr.deleted_at IS NULL AND r.deleted_at IS NULL;
