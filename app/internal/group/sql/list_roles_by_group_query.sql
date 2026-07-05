-- name: ListRolesByGroup :many
SELECT r.id, r.name, gr.id AS group_to_role_id
FROM role r
JOIN group_to_role gr ON gr.role_id = r.id
WHERE gr.group_id = :group_id
  AND gr.deleted_at IS NULL
  AND r.deleted_at IS NULL
ORDER BY r.name COLLATE NOCASE;
