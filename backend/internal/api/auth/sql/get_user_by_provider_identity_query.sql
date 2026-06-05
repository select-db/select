-- name: GetUserByProviderIdentity :one
SELECT
    u.id, u.email, u.name
FROM
    app.user_identity ui
    JOIN app."user" u ON u.id = ui.user_id
WHERE
    ui.provider = $1
    AND ui.provider_user_id = $2;
