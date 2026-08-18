DROP TABLE IF EXISTS verification_codes;
DROP TYPE IF EXISTS verification_purpose;

DROP INDEX IF EXISTS idx_users_personal_email;
DROP INDEX IF EXISTS idx_users_phone_number;

ALTER TABLE users
    DROP COLUMN IF EXISTS work_email_verified_at,
    DROP COLUMN IF EXISTS work_email_verified,
    DROP COLUMN IF EXISTS company_domain,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS personal_email,
    DROP COLUMN IF EXISTS phone_number;
