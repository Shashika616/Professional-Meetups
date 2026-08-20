-- Post-meetup star ratings (ADR-015). See docs/02-domain/domain-model.md's
-- "Rating" section for the full domain description — pairwise, 1-5 stars,
-- one per (meetup, rater, ratee) pair, anonymous to the ratee.

-- rating_average/rating_count live on users (not a separate services/meetup-
-- owned table) because services/meetup and services/auth already share this
-- one literal Postgres database (confirmed: both point at the same
-- DATABASE_URL) and services/meetup already reads host/requester display
-- info straight off this table — writing the aggregate here, in the same
-- transaction as the meetup_ratings insert, needs no Pub/Sub event (ADR-015).
ALTER TABLE users
    ADD COLUMN rating_average NUMERIC(3,2) NOT NULL DEFAULT 0,
    ADD COLUMN rating_count   INTEGER NOT NULL DEFAULT 0;

CREATE TABLE meetup_ratings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meetup_id       UUID NOT NULL REFERENCES meetups(id) ON DELETE CASCADE,
    rater_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rated_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    score           SMALLINT NOT NULL CHECK (score BETWEEN 1 AND 5),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Backstop against a malformed direct API call — the UI never offers
    -- self as a rateable option, so this only fires there (ADR-015).
    CHECK (rater_user_id <> rated_user_id),
    -- One rating per pair per meetup, immutable (no edit/re-rate) — a
    -- second SubmitRating call for the same pair hits this and maps to
    -- apperror.ErrConflict.
    UNIQUE (meetup_id, rater_user_id, rated_user_id)
);
CREATE INDEX idx_meetup_ratings_rated ON meetup_ratings (rated_user_id);
