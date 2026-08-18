-- name: GetUserByLinkedInSub :one
SELECT * FROM users WHERE linkedin_sub = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (linkedin_sub, full_name, profile_photo_url, headline, trust_level)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- Level 2/3 verification (ADR-012, backend/PLAN.md's matching addendum).
-- Each mutation also writes trust_level in the same statement — the caller
-- (internal/service) computes the new value via computeTrustLevel before
-- calling these, so the row is never left with a stale trust_level between
-- the field write and a separate recompute step.

-- name: UpdateUserPhoneNumber :one
UPDATE users SET phone_number = $2, trust_level = $3 WHERE id = $1 RETURNING *;

-- name: UpdateUserPersonalEmail :one
UPDATE users SET personal_email = $2, trust_level = $3 WHERE id = $1 RETURNING *;

-- name: UpdateUserPersonalDetails :one
UPDATE users SET legal_name = $2, address = $3, trust_level = $4 WHERE id = $1 RETURNING *;

-- name: UpdateUserWorkEmailVerified :one
UPDATE users SET company_domain = $2, work_email_verified = $3, work_email_verified_at = $4, trust_level = $5
WHERE id = $1 RETURNING *;
