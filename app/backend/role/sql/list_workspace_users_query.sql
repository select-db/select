-- name: ListWorkspaceUsers :many
SELECT
    u.id,
    u.name,
    u.email,
    CASE WHEN w.owner_id = u.id THEN 1 ELSE 0 END AS is_owner
FROM user u
JOIN workspace_to_user wtu ON wtu.user_id = u.id
JOIN workspace w ON w.id = wtu.workspace_id
WHERE wtu.workspace_id = :workspace_id
ORDER BY is_owner DESC, u.name COLLATE NOCASE;
