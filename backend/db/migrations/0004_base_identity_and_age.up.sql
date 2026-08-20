-- Base identity (Apple/Google/email+password), LinkedIn as the sole
-- trust-granting step, and mandatory 18+ self-attestation (ADR-014, final
-- shape 2026-08-19; backend/level0-federated-identity-PLAN.md Step 1).

CREATE TYPE identity_provider AS ENUM ('apple', 'google');
-- LinkedIn deliberately NOT in this enum/table — linkedin_sub stays on
-- `users` directly (ADR-014's Consequences), whether set at direct signup
-- or linked later via Profile, so there is exactly one place to check
-- "does this user have LinkedIn."

CREATE TABLE user_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    identity_provider NOT NULL,
    subject     TEXT NOT NULL,       -- the provider's stable 'sub' claim
    email       TEXT,                -- from the verified id_token, display-only, not an identity anchor
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (provider, subject),      -- one provider identity can't attach to two users
    UNIQUE (user_id, provider)       -- a user can't link the same provider twice
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);

ALTER TABLE users
    ADD COLUMN age_confirmed_over_18 BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN age_confirmed_at      TIMESTAMPTZ,
    -- Nullable: only ever set for an account that used the email+password
    -- path (signup, or recovery per SignUpOrRecoverWithEmail) — an
    -- Apple/Google/LinkedIn-only account has no password at all.
    ADD COLUMN password_hash         TEXT,
    ALTER COLUMN trust_level SET DEFAULT 0;  -- was 1; a fresh row before computeTrustLevel runs should reflect Level 0, not overclaim Level 1

-- Email+password signup (ADR-014 decision #2) reuses verification_codes
-- (ADR-012) rather than a parallel OTP system, per the plan's explicit
-- instruction not to build a second one — but every existing purpose
-- verifies an ALREADY-authenticated user's own field, keyed by
-- (user_id, purpose). Email signup has no user_id yet at OTP-send time —
-- the account may not even end up created, see SignUpOrRecoverWithEmail's
-- recovery path — so user_id must become nullable for this one purpose,
-- and a pending signup attempt is deduplicated by (purpose, target)
-- instead, via the partial unique index below (ordinary rows for the
-- other three purposes are untouched: user_id stays NOT NULL in practice
-- for those, just no longer enforced at the schema level).
ALTER TYPE verification_purpose ADD VALUE 'email_signup';
ALTER TABLE verification_codes ALTER COLUMN user_id DROP NOT NULL;
CREATE UNIQUE INDEX idx_verification_codes_signup_target
    ON verification_codes (purpose, target) WHERE user_id IS NULL;
