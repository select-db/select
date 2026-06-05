-- name: AddAPIKeyRole :exec
INSERT INTO auth.api_key_to_role (api_key_id, role_id)
VALUES ($1, $2)
ON CONFLICT (api_key_id, role_id) DO NOTHING;
