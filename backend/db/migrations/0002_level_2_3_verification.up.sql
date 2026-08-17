-- Level 2 (phone, personal email, personal details) and Level 3 (corporate
-- email) verification, per ADR-012 and backend/PLAN.md's matching addendum.
-- Mirrors the existing linkedin_sub pattern: presence of the value IS the
-- verified signal, no separate boolean where avoidable.

ALTER TABLE users
    ADD COLUMN phone_number TEXT,
    ADD COLUMN personal_email TEXT,
    ADD COLUMN legal_name TEXT,
    ADD COLUMN address TEXT,
    -- Extracted from a verified corporate email, never the raw address
    -- (ADR-003, unchanged).
    ADD COLUMN company_domain TEXT,
    ADD COLUMN work_email_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN work_email_verified_at TIMESTAMPTZ;

-- Real secondary identity anchors (ADR-012), not just verified profile
-- fields — non-null values must be unique platform-wide.
CREATE UNIQUE INDEX idx_users_phone_number ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE UNIQUE INDEX idx_users_personal_email ON users (personal_email) WHERE personal_email IS NOT NULL;

CREATE TYPE verification_purpose AS ENUM ('phone', 'personal_email', 'corporate_email');

CREATE TABLE verification_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     verification_purpose NOT NULL,
    -- The phone number or email address being verified; deleted once
    -- verified (ADR-003 for the corporate-email case) or expired.
    target      TEXT NOT NULL,
    -- SHA-256 of the OTP, same pattern as refresh_tokens.token_hash.
    code_hash   TEXT NOT NULL,
    attempts    SMALLINT NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One pending code per user per purpose — a new send overwrites
    -- (upsert), it doesn't stack. Also what makes the 1-minute resend timer
    -- server-enforceable: "is there already a non-expired code for this
    -- user+purpose, and how old is it" is one indexed lookup.
    UNIQUE (user_id, purpose)
);
