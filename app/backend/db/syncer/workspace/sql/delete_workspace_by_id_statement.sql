-- name: DeleteWorkspaceByID :exec
; -- @no-track
DELETE FROM workspace
WHERE id = :id;
