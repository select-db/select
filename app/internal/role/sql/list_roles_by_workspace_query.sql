-- name: ListRolesByWorkspace :many
SELECT
    r.id,
    r.workspace_id,
    r.name,
    (SELECT COUNT(*) FROM user_to_role ur WHERE ur.role_id = r.id) AS user_count,
    (SELECT COUNT(*) FROM permission p WHERE p.role_id = r.id) AS permission_count,
    (SELECT COUNT(*) FROM group_to_role gr WHERE gr.role_id = r.id AND gr.deleted_at IS NULL) AS group_count
FROM role r
WHERE r.workspace_id = :workspace_id
ORDER BY r.name COLLATE NOCASE;
