-- name: ListGroupsByWorkspace :many
SELECT
    g.id,
    g.workspace_id,
    g.name,
    (SELECT COUNT(*) FROM user_to_group ug WHERE ug.group_id = g.id AND ug.deleted_at IS NULL) AS member_count,
    (SELECT COUNT(*) FROM group_to_role gr WHERE gr.group_id = g.id AND gr.deleted_at IS NULL) AS role_count
FROM "group" g
WHERE g.workspace_id = :workspace_id AND g.deleted_at IS NULL
ORDER BY g.name COLLATE NOCASE;
