DROP INDEX IF EXISTS idx_meetup_ratings_rated;
DROP TABLE IF EXISTS meetup_ratings;

ALTER TABLE users
    DROP COLUMN IF EXISTS rating_average,
    DROP COLUMN IF EXISTS rating_count;
