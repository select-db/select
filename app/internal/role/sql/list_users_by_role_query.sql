-- name: ListUsersByRole :many
SELECT u.id, u.name, utr.id AS user_to_role_id
FROM user u
JOIN user_to_role utr ON utr.user_id = u.id
WHERE utr.role_id = :role_id
  AND utr.deleted_at IS NULL
ORDER BY u.name COLLATE NOCASE;
