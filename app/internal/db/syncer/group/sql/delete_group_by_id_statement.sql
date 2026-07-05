-- name: DeleteGroupByID :exec
; -- @no-track
DELETE FROM "group" WHERE id = :id;
