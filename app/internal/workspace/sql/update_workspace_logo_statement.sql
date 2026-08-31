-- name: UpdateWorkspaceLogo :exec
; -- @no-track
UPDATE workspace
SET logo = :logo
WHERE id = :id;
