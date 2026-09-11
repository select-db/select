-- name: UpdateCurrentWorkspaceToUser :exec
; -- @no-track
-- One statement, because GetCurrentUser selects on this flag: clearing the old
-- row before setting the new one leaves a window with no current user at all,
-- and anything asking in it is told there is none.
UPDATE workspace_to_user
SET current = (workspace_id = :workspace_id)
WHERE user_id = :user_id;
