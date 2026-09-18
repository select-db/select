-- name: ListUserGroupsByWorkspace :many
SELECT ug.user_id, ug.id AS user_to_group_id, g.id AS group_id, g.name AS group_name
FROM user_to_group ug
JOIN "group" g ON g.id = ug.group_id
WHERE ug.workspace_id = :workspace_id
  AND ug.deleted_at IS NULL
  AND g.deleted_at IS NULL
ORDER BY g.name COLLATE NOCASE;
