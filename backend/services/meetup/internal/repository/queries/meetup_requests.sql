-- name: CreateMeetupRequest :one
INSERT INTO meetup_requests (meetup_id, requester_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetMeetupRequestByID :one
-- Plain, no join — used inside Accept's transaction where requester
-- display info is irrelevant, and as the base row write queries (Accept/
-- Reject/Withdraw) RETURNING against. Callers needing display info should
-- use GetMeetupRequestWithRequesterInfoByID instead.
SELECT * FROM meetup_requests WHERE id = $1;

-- name: GetMeetupRequestWithRequesterInfoByID :one
SELECT
  r.*,
  u.full_name AS requester_full_name,
  u.profile_photo_url AS requester_profile_photo_url,
  u.trust_level AS requester_trust_level,
  u.rating_average AS requester_rating_average,
  u.rating_count AS requester_rating_count
FROM meetup_requests r
JOIN users u ON u.id = r.requester_id
WHERE r.id = $1;

-- name: ListRequestsForMeetup :many
SELECT
  r.*,
  u.full_name AS requester_full_name,
  u.profile_photo_url AS requester_profile_photo_url,
  u.trust_level AS requester_trust_level,
  u.rating_average AS requester_rating_average,
  u.rating_count AS requester_rating_count
FROM meetup_requests r
JOIN users u ON u.id = r.requester_id
WHERE r.meetup_id = $1
ORDER BY r.created_at ASC;

-- name: CountAcceptedRequests :one
SELECT count(*) FROM meetup_requests WHERE meetup_id = $1 AND status = 'accepted';

-- name: AcceptMeetupRequest :one
UPDATE meetup_requests SET status = 'accepted', resolved_at = now()
WHERE id = $1
RETURNING *;

-- name: RejectMeetupRequest :one
-- WHERE status = 'pending' guards against rejecting an already-resolved
-- request (double-tap, or a race with auto-reject) — zero rows affected
-- (pgx.ErrNoRows) is the repository's signal to map to apperror.ErrConflict.
UPDATE meetup_requests SET status = 'rejected', resolved_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: AutoRejectPendingRequestsForMeetup :many
-- Returns the rejected rows so the caller can notify each requester
-- (backend/meetup-scheduling-PLAN.md Step E).
UPDATE meetup_requests SET status = 'rejected', resolved_at = now(), auto_rejected = true
WHERE meetup_id = $1 AND status = 'pending'
RETURNING *;

-- name: WithdrawMeetupRequest :one
UPDATE meetup_requests SET status = 'withdrawn', resolved_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;
