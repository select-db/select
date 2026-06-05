-- name: UpdateWorkspaceGitRemote :exec
UPDATE workspace
SET git_remote_url = :git_remote_url
WHERE id = :id;
