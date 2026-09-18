-- name: GetAPIKeyRolesWithNames :many
SELECT r.id, r.name
FROM app.role r
JOIN auth.api_key_to_role atr ON atr.role_id = r.id
WHERE atr.api_key_id = $1 AND r.deleted_at IS NULL;
