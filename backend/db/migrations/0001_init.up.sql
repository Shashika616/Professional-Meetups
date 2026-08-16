-- Initial schema for the LinkedIn federated (Level 1a) onboarding slice only.
-- Deliberately minimal: phone, personal email, personal details, corporate
-- email, and KYC columns (Levels 2-4, ADR-006) are NOT added here. They
-- arrive in their own migration when that slice is actually built — adding
-- unused nullable columns "for later" is exactly the kind of schema drift
-- this project is trying to avoid. See docs/04-decisions/adr-011.

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for gen_random_uuid()

CREATE TYPE account_status AS ENUM ('active', 'deactivated', 'deleted');

CREATE TABLE users (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The OIDC 'sub' claim LinkedIn returns — the durable, stable identifier
    -- for a LinkedIn account. NULL only if we ever build the Level 1b
    -- "claimed URL" fallback (not this slice). Presence of a value here IS
    -- the signal that this user completed federated LinkedIn OAuth — there
    -- is deliberately no separate "linkedin_connected" boolean duplicating
    -- that fact.
    linkedin_sub       TEXT UNIQUE,

    full_name          TEXT NOT NULL,
    profile_photo_url  TEXT,
    headline           TEXT, -- LinkedIn headline, e.g. "Software Engineer at Acme"

    trust_level        SMALLINT NOT NULL DEFAULT 1 CHECK (trust_level BETWEEN 0 AND 4),
    account_status     account_status NOT NULL DEFAULT 'active',

    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup on login/callback; partial index since linkedin_sub is nullable
-- and we only ever look up by it when it's present.
CREATE UNIQUE INDEX idx_users_linkedin_sub ON users (linkedin_sub) WHERE linkedin_sub IS NOT NULL;

CREATE TABLE refresh_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- SHA-256 of the actual refresh token, hex-encoded. The raw token is
    -- returned to the client exactly once and never stored — this table can
    -- only recognize a presented token, never reproduce it (ADR-009).
    token_hash    TEXT NOT NULL UNIQUE,

    issued_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,

    -- Set when this token was rotated out for a newer one. A presented token
    -- whose row already has replaced_by set is a replay of an old,
    -- already-rotated token — treat as a theft signal (ADR-009).
    replaced_by   UUID REFERENCES refresh_tokens(id)
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);

-- updated_at should always reflect the last write, without every call site
-- having to remember to set it by hand.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
