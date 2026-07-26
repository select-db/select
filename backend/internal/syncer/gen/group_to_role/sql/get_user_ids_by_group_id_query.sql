-- name: GetUserIDsByGroupID :many
SELECT user_id
FROM app.user_to_group
WHERE group_id = $1 AND deleted_at IS NULL;
