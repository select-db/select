-- name: ListUsersByGroup :many
SELECT u.id, u.name, ug.id AS user_to_group_id
FROM user u
JOIN user_to_group ug ON ug.user_id = u.id
WHERE ug.group_id = :group_id
  AND ug.deleted_at IS NULL
ORDER BY u.name COLLATE NOCASE;
