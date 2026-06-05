-- name: UpsertUserForSync :exec
; -- @no-track
INSERT INTO user (id, name, email, avatar_url)
VALUES (:id, :name, :email, :avatar_url)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    email = excluded.email,
    avatar_url = excluded.avatar_url;
