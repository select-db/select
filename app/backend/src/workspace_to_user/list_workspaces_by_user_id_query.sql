-- name: ListWorkspacesByUserID :many
SELECT
    w.id,
    w.name,
    wtu.current
FROM
    workspace w
    INNER JOIN workspace_to_user wtu ON wtu.workspace_id = w.id
WHERE
    wtu.user_id = :user_id
ORDER BY
    w.name ASC;
