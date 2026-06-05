-- name: GetRoleWorkspaceID :one
SELECT workspace_id FROM role WHERE id = :id;
