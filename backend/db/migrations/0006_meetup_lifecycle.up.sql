-- Meetup time windows and host-initiated lifecycle closing (ADR-016).
-- Corrects ADR-013's single-instant scheduled_for and revives meetup_status's
-- 'completed' value, left deliberately unused by ADR-015 (that position is
-- now superseded — see ADR-016's Context section).

ALTER TABLE meetups
    ADD COLUMN window_start TIMESTAMPTZ,
    ADD COLUMN window_end   TIMESTAMPTZ,
    ADD COLUMN closed_at    TIMESTAMPTZ;

-- Backfill existing rows before tightening to NOT NULL — anything currently
-- NULL (today/now path) or with only scheduled_for set gets a synthetic
-- window so the NOT NULL below doesn't fail against real data.
UPDATE meetups SET
    window_start = COALESCE(scheduled_for, created_at),
    window_end   = COALESCE(scheduled_for, created_at) + INTERVAL '2 hours'
WHERE window_start IS NULL;

ALTER TABLE meetups
    ALTER COLUMN window_start SET NOT NULL,
    ALTER COLUMN window_end SET NOT NULL,
    ADD CONSTRAINT meetups_window_valid CHECK (window_end > window_start),
    DROP COLUMN scheduled_for;

ALTER TABLE meetup_feedback
    ADD COLUMN notes TEXT;
