-- name: GetUserByEmail :one
SELECT
  u.id,
  u.email,
  u.name,
  EXISTS (
    SELECT
      1
    FROM
      app.workspace_to_user wtu
    WHERE
      wtu.user_id = u.id
      AND wtu.workspace_id = $2
      AND wtu.deleted_at IS NULL
  ) AS is_member
FROM
  app."user" u
  LEFT JOIN app.user_identity ui ON ui.user_id = u.id
WHERE
  LOWER(u.email) = LOWER($1)
  OR LOWER(ui.email) = LOWER($1)
LIMIT
  1;