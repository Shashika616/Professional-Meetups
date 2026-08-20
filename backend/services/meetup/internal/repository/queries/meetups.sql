-- name: CreateMeetup :one
INSERT INTO meetups (host_user_id, intent, window_start, window_end, location_lat, location_lng, location_label, capacity)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMeetupByID :one
-- my_request_status is the *latest* request this specific user (viewerID)
-- has made on this meetup, or NULL if they never requested — the frontend
-- uses this to render "REQUEST TO JOIN" vs. the request's current status
-- (frontend/meetup-scheduling-PLAN.md Step 3). Plain LEFT JOIN + ORDER BY
-- ... LIMIT 1, not a scalar subquery or LEFT JOIN LATERAL in the SELECT
-- list — sqlc's nullability analysis correctly infers a plain LEFT JOIN
-- column as nullable (NullMeetupRequestStatus); both of the other two forms
-- were confirmed (by generating each and inspecting the output) to produce
-- a non-nullable Go type that would panic scanning a real NULL row — a
-- viewer who's never requested to join, the common browsing case.
SELECT
  m.*,
  u.full_name AS host_full_name,
  u.profile_photo_url AS host_profile_photo_url,
  u.trust_level AS host_trust_level,
  u.rating_average AS host_rating_average,
  u.rating_count AS host_rating_count,
  (SELECT count(*) FROM meetup_requests r2 WHERE r2.meetup_id = m.id AND r2.status = 'accepted') AS accepted_count,
  r.status AS my_request_status
FROM meetups m
JOIN users u ON u.id = m.host_user_id
LEFT JOIN meetup_requests r ON r.meetup_id = m.id AND r.requester_id = $2
WHERE m.id = $1
ORDER BY r.created_at DESC NULLS LAST
LIMIT 1;

-- name: GetMeetupByIDForUpdate :one
-- Plain, no join — used only inside AcceptRequest's transaction to lock the
-- row and re-check status/capacity, host display info is irrelevant there.
SELECT * FROM meetups WHERE id = $1 FOR UPDATE;

-- name: ListOpenMeetupsByIntentFirstPage :many
-- DISTINCT ON (m.id) + a wrapping SELECT to re-sort by recency: DISTINCT ON
-- requires its own ORDER BY to start with the same expression(s), which
-- would otherwise force sorting this page by id instead of created_at —
-- the inner query dedupes (a requester can have multiple historical
-- meetup_requests rows for the same meetup, see ListMeetupsRequestedByUser
-- above), the outer one restores the intended pagination order. Plain LEFT
-- JOIN (not LATERAL/a scalar subquery) for my_request_status, same
-- nullability reasoning as GetMeetupByID above.
WITH deduped AS (
  SELECT DISTINCT ON (m.id)
    m.*,
    u.full_name AS host_full_name,
    u.profile_photo_url AS host_profile_photo_url,
    u.trust_level AS host_trust_level,
    u.rating_average AS host_rating_average,
    u.rating_count AS host_rating_count,
    (SELECT count(*) FROM meetup_requests r2 WHERE r2.meetup_id = m.id AND r2.status = 'accepted') AS accepted_count,
    r.status AS my_request_status
  FROM meetups m
  JOIN users u ON u.id = m.host_user_id
  LEFT JOIN meetup_requests r ON r.meetup_id = m.id AND r.requester_id = $2
  WHERE m.status = 'open' AND m.intent = $1
  ORDER BY m.id, r.created_at DESC NULLS LAST
)
SELECT * FROM deduped
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: ListOpenMeetupsByIntentAfterCursor :many
-- Keyset pagination on (created_at, id) — cursorCreatedAt/cursorID are the
-- last row of the previous page, so this resumes strictly after it. Row
-- comparison (a, b) < (c, d) is a single index-friendly condition, not a
-- chain of ORs. Same DISTINCT-then-re-sort shape as the first-page query
-- above, for the same reason.
WITH deduped AS (
  SELECT DISTINCT ON (m.id)
    m.*,
    u.full_name AS host_full_name,
    u.profile_photo_url AS host_profile_photo_url,
    u.trust_level AS host_trust_level,
    u.rating_average AS host_rating_average,
    u.rating_count AS host_rating_count,
    (SELECT count(*) FROM meetup_requests r2 WHERE r2.meetup_id = m.id AND r2.status = 'accepted') AS accepted_count,
    r.status AS my_request_status
  FROM meetups m
  JOIN users u ON u.id = m.host_user_id
  LEFT JOIN meetup_requests r ON r.meetup_id = m.id AND r.requester_id = $2
  WHERE m.status = 'open' AND m.intent = $1
  ORDER BY m.id, r.created_at DESC NULLS LAST
)
SELECT * FROM deduped
WHERE (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: ListMeetupsByHost :many
SELECT
  m.*,
  u.full_name AS host_full_name,
  u.profile_photo_url AS host_profile_photo_url,
  u.trust_level AS host_trust_level,
  u.rating_average AS host_rating_average,
  u.rating_count AS host_rating_count,
  (SELECT count(*) FROM meetup_requests r WHERE r.meetup_id = m.id AND r.status = 'accepted') AS accepted_count
FROM meetups m
JOIN users u ON u.id = m.host_user_id
WHERE m.host_user_id = $1
ORDER BY m.created_at DESC;

-- name: ListMeetupsRequestedByUser :many
-- One row per meetup, carrying the requester's *latest* request status for
-- it (a requester can have multiple historical rows for the same meetup —
-- e.g. rejected, then withdrawn, then a fresh pending one — the UNIQUE
-- constraint is per (meetup_id, requester_id, status), not per
-- (meetup_id, requester_id) alone).
WITH latest_request AS (
  SELECT DISTINCT ON (meetup_id) *
  FROM meetup_requests
  WHERE meetup_requests.requester_id = $1
  ORDER BY meetup_id, created_at DESC
)
SELECT
  m.*,
  u.full_name AS host_full_name,
  u.profile_photo_url AS host_profile_photo_url,
  u.trust_level AS host_trust_level,
  u.rating_average AS host_rating_average,
  u.rating_count AS host_rating_count,
  (SELECT count(*) FROM meetup_requests r2 WHERE r2.meetup_id = m.id AND r2.status = 'accepted') AS accepted_count,
  lr.status AS my_request_status,
  lr.auto_rejected AS my_request_auto_rejected
FROM latest_request lr
JOIN meetups m ON m.id = lr.meetup_id
JOIN users u ON u.id = m.host_user_id
ORDER BY m.created_at DESC;

-- name: MarkMeetupFull :exec
UPDATE meetups SET status = 'full' WHERE id = $1;

-- name: CancelMeetup :one
UPDATE meetups SET status = 'cancelled', cancelled_at = now()
WHERE id = $1
RETURNING *;

-- name: CloseMeetup :one
-- The WHERE clause's four conditions (right meetup, right host, currently
-- open-ish, window actually started) are the *entire* authorization and
-- precondition check — done in the query itself so there's no window
-- between "check" and "act" for a concurrent request to slip through
-- (ADR-016). Zero rows updated (vs. an error) means one of those four
-- failed; the service layer re-fetches to distinguish which for a useful
-- error message.
UPDATE meetups
SET status = 'completed', closed_at = now()
WHERE id = $1 AND host_user_id = $2 AND status IN ('open', 'full') AND now() >= window_start
RETURNING *;
