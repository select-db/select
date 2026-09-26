-- name: GetWorkspacePlan :one
SELECT
  w.plan,
  (
    SELECT
      count(*)
    FROM
      app.workspace_to_user wtu
    WHERE
      wtu.workspace_id = w.id
      AND wtu.deleted_at IS NULL
  ) AS members
FROM
  app.workspace w
WHERE
  w.id = $1
  AND w.deleted_at IS NULL;
