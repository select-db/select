-- name: InsertWorkspaceToUserForAdd :one
INSERT INTO
  app.workspace_to_user (id, workspace_id, user_id, updated_at)
VALUES
  (gen_random_uuid (), $1, $2, NOW())
RETURNING
  id;