-- name: GetUserByLinkedInSub :one
SELECT * FROM users WHERE linkedin_sub = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByPersonalEmail :one
-- personal_email's mere presence already means "verified" in this schema
-- (same "presence IS the signal" convention as linkedin_sub/phone_number —
-- it's only ever written via UpdateUserPersonalEmail, which only runs
-- after a successful OTP check) — no separate boolean to check here.
-- Used by SignUpOrRecoverWithEmail (ADR-014 decision #3) to detect the
-- recovery case: an email+password signup against an address that's
-- already someone's verified personal_email.
SELECT * FROM users WHERE personal_email = $1;

-- name: CreateUser :one
-- age_confirmed_at is set to now() only when age_confirmed_over_18 is true
-- (the only case CreateUser is ever called with, in practice — the service
-- layer rejects false before reaching here) — never backdated, never set
-- for a false confirmation.
INSERT INTO users (linkedin_sub, full_name, profile_photo_url, headline, trust_level, age_confirmed_over_18, age_confirmed_at)
VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $6 THEN now() ELSE NULL END)
RETURNING *;

-- name: UpdateUserLinkedInSub :one
-- The LinkedIn branch of LinkIdentityToUser (ADR-014) — links LinkedIn to
-- an already-authenticated Level 0+ account (Profile's "Connect LinkedIn").
-- Direct LinkedIn signup (CompleteLinkedInOnboarding, unchanged by this
-- slice) still creates accounts via CreateUser directly; this query is
-- only for the linking-to-an-existing-account path. The partial unique
-- index idx_users_linkedin_sub (migration 0001) is what actually rejects
-- linking a LinkedIn subject already claimed by a different user — the
-- caller (internal/service) maps that 23505 into apperror.ErrConflict,
-- same pattern as UpdateUserPhoneNumber/UpdateUserPersonalEmail below.
UPDATE users SET linkedin_sub = $2, trust_level = $3 WHERE id = $1 RETURNING *;

-- name: SetUserPasswordHash :one
-- Used both by fresh email+password signup and by
-- SignUpOrRecoverWithEmail's recovery path (ADR-014 decision #3) — setting
-- a password on an existing account is the entire recovery mechanism,
-- there's no separate "recover" mutation. hash is always an argon2id hash,
-- never the raw password (internal/service's password.go).
UPDATE users SET password_hash = $2 WHERE id = $1 RETURNING *;

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
