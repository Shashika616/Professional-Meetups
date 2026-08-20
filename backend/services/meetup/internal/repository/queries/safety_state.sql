-- name: EnsureSafetyState :exec
-- Idempotent create-if-missing — called the moment a meetup gets its first
-- accepted request (ADR-013 § 3). ON CONFLICT DO NOTHING rather than
-- upsert-with-RETURNING since callers always follow this with a plain Get.
INSERT INTO meetup_safety_state (meetup_id) VALUES ($1)
ON CONFLICT (meetup_id) DO NOTHING;

-- name: GetSafetyState :one
SELECT * FROM meetup_safety_state WHERE meetup_id = $1;

-- name: SetChecklistAck :one
UPDATE meetup_safety_state SET checklist_ack_at = now() WHERE meetup_id = $1
RETURNING *;

-- name: SetLiveLocationOptIn :one
UPDATE meetup_safety_state SET live_location_opt_in = $2 WHERE meetup_id = $1
RETURNING *;

-- name: SetCheckedIn :one
UPDATE meetup_safety_state SET checked_in_at = now() WHERE meetup_id = $1
RETURNING *;
