-- name: UpsertDeviceToken :one
-- Upserts by token, not by user — a token identifies one physical device
-- install; re-registering it under a different account reassigns
-- ownership rather than leaving a stale row.
INSERT INTO device_tokens (user_id, fcm_token)
VALUES ($1, $2)
ON CONFLICT (fcm_token) DO UPDATE SET user_id = $1, updated_at = now()
RETURNING *;

-- name: ListDeviceTokensForUser :many
SELECT * FROM device_tokens WHERE user_id = $1;
