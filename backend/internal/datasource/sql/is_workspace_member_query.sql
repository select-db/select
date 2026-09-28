-- name: IsWorkspaceMember :one
SELECT
  EXISTS (
    SELECT 1 FROM app.workspace_to_user
    WHERE workspace_id = $1 AND user_id = $2 AND deleted_at IS NULL
  );
