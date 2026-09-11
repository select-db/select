-- name: GetCurrentWorkspaceToUser :one
-- Joined on the signed-in user, because the current flag is cleared per user:
-- a row left current by whoever was signed in before would otherwise win.
SELECT
    wtu.*
FROM
    workspace_to_user wtu
    JOIN user u ON u.id = wtu.user_id
WHERE
    wtu.current = TRUE
    AND u.current = TRUE
LIMIT 1;
