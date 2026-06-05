-- name: ListUserRolesByWorkspace :many
SELECT utr.user_id, utr.id AS user_to_role_id, r.id AS role_id, r.name AS role_name
FROM user_to_role utr
JOIN role r ON r.id = utr.role_id
WHERE utr.workspace_id = :workspace_id
  AND utr.deleted_at IS NULL
  AND r.deleted_at IS NULL
ORDER BY r.name COLLATE NOCASE;
