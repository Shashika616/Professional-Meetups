-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: GetRefreshTokenByID :one
SELECT * FROM refresh_tokens WHERE id = $1;

-- name: MarkRefreshTokenReplaced :exec
UPDATE refresh_tokens SET replaced_by = $2
WHERE id = $1;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL;
