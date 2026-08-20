DROP INDEX IF EXISTS idx_verification_codes_signup_target;
-- Fails if any row currently has a NULL user_id (an in-flight, never-
-- completed email signup) — acceptable for a dev-only rollback path, same
-- simplicity precedent as this migration set's other .down.sql files.
ALTER TABLE verification_codes ALTER COLUMN user_id SET NOT NULL;
-- Postgres has no DROP VALUE for enums — 'email_signup' stays defined on
-- the type after rollback (harmless, unused) rather than requiring a full
-- enum rebuild just to remove it.

ALTER TABLE users
    ALTER COLUMN trust_level SET DEFAULT 1,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS age_confirmed_at,
    DROP COLUMN IF EXISTS age_confirmed_over_18;

DROP INDEX IF EXISTS idx_user_identities_user_id;
DROP TABLE IF EXISTS user_identities;
DROP TYPE IF EXISTS identity_provider;
