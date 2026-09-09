-- name: ClearWorkspaceLocalPaths :exec
; -- @no-track
UPDATE workspace
SET local_path = NULL
WHERE local_path IS NOT NULL;
