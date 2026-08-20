-- name: IsMeetupParticipant :one
-- True if userID is the host of meetupID, or has an accepted request on it
-- — the "ratable set" (ADR-015, docs/02-domain/domain-model.md § Rating).
SELECT EXISTS(
  SELECT 1 FROM meetups m WHERE m.id = sqlc.arg(meetup_id) AND m.host_user_id = sqlc.arg(user_id)
  UNION
  SELECT 1 FROM meetup_requests r WHERE r.meetup_id = sqlc.arg(meetup_id) AND r.requester_id = sqlc.arg(user_id) AND r.status = 'accepted'
);

-- name: HasConfirmedMeetupHappened :one
-- The rating-eligibility gate: the *rater* must have already confirmed
-- (SubmitMeetupFeedback, happened=true) that the meetup happened. The ratee
-- does not need this — a no-show is legitimately ratable by someone who did
-- attend and confirm (ADR-015).
SELECT EXISTS(
  SELECT 1 FROM meetup_feedback WHERE meetup_id = $1 AND user_id = $2 AND happened = true
);

-- name: ListRatableParticipants :many
-- Host + accepted requesters of meetupID, excluding viewerID, each flagged
-- with whether viewerID has already rated them for this meetup.
SELECT
  u.id,
  u.full_name,
  u.profile_photo_url,
  u.trust_level,
  (mr.id IS NOT NULL)::boolean AS already_rated
FROM (
  SELECT m.host_user_id AS user_id FROM meetups m WHERE m.id = sqlc.arg(meetup_id)
  UNION
  SELECT r.requester_id AS user_id FROM meetup_requests r WHERE r.meetup_id = sqlc.arg(meetup_id) AND r.status = 'accepted'
) participants
JOIN users u ON u.id = participants.user_id
LEFT JOIN meetup_ratings mr
  ON mr.meetup_id = sqlc.arg(meetup_id) AND mr.rater_user_id = sqlc.arg(viewer_id) AND mr.rated_user_id = participants.user_id
WHERE participants.user_id <> sqlc.arg(viewer_id);

-- name: CreateMeetupRating :one
INSERT INTO meetup_ratings (meetup_id, rater_user_id, rated_user_id, score)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RecomputeUserRating :exec
-- Full recompute via AVG()/COUNT() over meetup_ratings, not an incremental
-- running update — self-correcting, avoids drift, and (run inside the same
-- transaction as CreateMeetupRating, after it) the row lock this UPDATE
-- takes on users serializes concurrent raters of the same person correctly
-- (ADR-015).
UPDATE users
SET rating_count = (SELECT count(*) FROM meetup_ratings mr WHERE mr.rated_user_id = users.id),
    rating_average = (SELECT COALESCE(ROUND(AVG(mr.score), 2), 0) FROM meetup_ratings mr WHERE mr.rated_user_id = users.id)
WHERE users.id = $1;
