-- name: SetCurrentUser :exec
; -- @no-track
-- One statement: anything asking who is signed in between a clear and a set
-- would be told nobody is.
UPDATE user
SET current = (id = :id);
