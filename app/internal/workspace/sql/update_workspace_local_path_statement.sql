-- name: UpdateWorkspaceLocalPath :exec
; -- @no-track
UPDATE workspace
SET local_path = :local_path
WHERE id = :id;
