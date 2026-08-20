ALTER TABLE meetup_feedback DROP COLUMN IF EXISTS notes;

ALTER TABLE meetups
    DROP CONSTRAINT IF EXISTS meetups_window_valid,
    ADD COLUMN scheduled_for TIMESTAMPTZ;

-- Backfill scheduled_for from window_start — lossy (window_end/closed_at
-- are simply dropped below, and a genuinely-NULL-meant-"now" original value
-- can't be distinguished from a real backfilled instant any more), same
-- acceptable dev-stage-rollback precedent as 0004's down-migration.
UPDATE meetups SET scheduled_for = window_start;

ALTER TABLE meetups
    DROP COLUMN IF EXISTS window_start,
    DROP COLUMN IF EXISTS window_end,
    DROP COLUMN IF EXISTS closed_at;
