-- name: UpsertVerificationCode :one
-- One pending code per (user_id, purpose) — a resend overwrites the
-- existing row (including resetting attempts and created_at) rather than
-- stacking a second one.
INSERT INTO verification_codes (user_id, purpose, target, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, purpose)
DO UPDATE SET target = $3, code_hash = $4, attempts = 0, expires_at = $5, created_at = now()
RETURNING *;

-- name: GetVerificationCode :one
SELECT * FROM verification_codes WHERE user_id = $1 AND purpose = $2;

-- name: IncrementVerificationAttempts :one
UPDATE verification_codes SET attempts = attempts + 1
WHERE user_id = $1 AND purpose = $2
RETURNING *;

-- name: DeleteVerificationCode :exec
DELETE FROM verification_codes WHERE user_id = $1 AND purpose = $2;
