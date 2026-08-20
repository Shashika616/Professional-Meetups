# Backend PLAN — Meetup Time Windows, Host-Initiated Closing, Feedback Notes, LoginWithPassword Timing Fix

Implements ADR-016, plus one unrelated security fix bundled in at Shashika's request. Read ADR-016 first. Scope: `services/meetup` (time windows, lifecycle, feedback notes) and `services/auth` (timing fix) — two independent changes in one handoff, not one feature.

## Part A — `services/auth`: LoginWithPassword timing fix

`internal/service/service.go`'s `LoginWithPassword`:

```go
// current — short-circuits on an empty PasswordHash, skipping verifyPassword
// entirely, so a federated-only account returns near-instantly while a real
// wrong-password attempt pays the full argon2id cost first. Same error
// message either way, but not the same timing — a measurable side channel
// that undercuts this function's own "enumeration-safe" doc comment.
if user.PasswordHash == "" || !verifyPassword(user.PasswordHash, req.GetPassword()) {
```

Fix: always call `verifyPassword` — against a constant dummy hash when `PasswordHash` is empty, so both paths pay the same cost:

```go
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=1,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // never a real hash, exists only so timing is uniform

hash := user.PasswordHash
if hash == "" {
    hash = dummyPasswordHash
}
if !verifyPassword(hash, req.GetPassword()) || user.PasswordHash == "" {
    return nil, apperror.ToGRPCStatus(fmt.Errorf("invalid email or password: %w", apperror.ErrUnauthorized))
}
```

(The trailing `|| user.PasswordHash == ""` after the constant-time comparison still correctly rejects federated-only accounts — it just no longer *skips* the comparison, so timing doesn't reveal which case it was.) Add a test asserting both paths call `verifyPassword` (e.g. via a call-counter or timing-insensitive assertion on the code path, not a literal timing measurement, which would be flaky in CI).

## Part B — `services/meetup`: time windows

### Migration `0006_meetup_lifecycle.up.sql`

```sql
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
```

Write the matching `.down.sql` (re-add `scheduled_for` nullable, backfill from `window_start`, drop the new columns — document the same "acceptable dev-stage lossiness" precedent already used in `0004`'s down-migration for an analogous case).

### `services/meetup/internal/repository/queries/meetups.sql`

Update `CreateMeetup`/`GetMeetup`/`ListOpenMeetups`/etc. to select `window_start`, `window_end`, `closed_at` instead of `scheduled_for`. New query:

```sql
-- name: CloseMeetup :one
UPDATE meetups
SET status = 'completed', closed_at = now()
WHERE id = $1 AND host_user_id = $2 AND status IN ('open', 'full') AND now() >= window_start
RETURNING *;
```

The `WHERE` clause's four conditions (right meetup, right host, currently open-ish, window actually started) are the *entire* authorization and precondition check — done in the query itself so there's no window between "check" and "act" for a concurrent request to slip through. Zero rows updated (vs. an error) means one of those four failed; the service layer below distinguishes which for a useful error message.

### `services/meetup/internal/service/meetup.go` (or wherever `CreateMeetup`/`CancelMeetup` already live)

`CreateMeetup`: validate `window_end > window_start` server-side too (defense in depth — the DB `CHECK` is the backstop, not the only check) and that `window_start` isn't in the past by more than a small grace window (a few minutes, to tolerate clock skew and request latency — reject anything clearly stale, like a window from yesterday).

New `CloseMeetup(ctx, meetupID, hostUserID)`:

```go
func (s *Service) CloseMeetup(ctx context.Context, req *meetupv1.CloseMeetupRequest) (*meetupv1.CloseMeetupResponse, error) {
    meetup, err := s.meetups.Close(ctx, req.GetMeetupId(), req.GetHostUserId())
    if err != nil {
        if errors.Is(err, apperror.ErrNotFound) {
            // CloseMeetup query returned zero rows — distinguish *why* for a
            // useful client message rather than a bare 404, without a second
            // round-trip: re-fetch the meetup (already need it for the
            // "not the host" vs "not open" vs "hasn't started" distinction).
            current, getErr := s.meetups.Get(ctx, req.GetMeetupId())
            if getErr != nil {
                return nil, apperror.ToGRPCStatus(getErr)
            }
            switch {
            case current.HostUserID != req.GetHostUserId():
                return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: only the host can close this meetup: %w", apperror.ErrForbidden))
            case current.Status != "open" && current.Status != "full":
                return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: already closed or cancelled: %w", apperror.ErrConflict))
            case time.Now().Before(current.WindowStart):
                return nil, apperror.ToGRPCStatus(fmt.Errorf("meetup: can't close before the meetup's window has started: %w", apperror.ErrForbidden))
            }
        }
        return nil, apperror.ToGRPCStatus(err)
    }
    return &meetupv1.CloseMeetupResponse{Meetup: meetupToProto(meetup)}, nil
}
```

`SubmitMeetupFeedback`: thread through the new optional `notes` field, no other logic change.

## Part C — proto (`backend/proto/meetup/v1/meetup.proto`)

```protobuf
rpc CloseMeetup(CloseMeetupRequest) returns (CloseMeetupResponse);
```

```protobuf
message CloseMeetupRequest {
  string meetup_id = 1;
  string host_user_id = 2; // set by the gateway from the verified JWT, never client-supplied
}
message CloseMeetupResponse {
  MeetupResponse meetup = 1;
}
```

`CreateMeetupRequest`: replace `optional int64 scheduled_for_unix_seconds = 4;` with:
```protobuf
int64 window_start_unix_seconds = 4;
int64 window_end_unix_seconds = 9; // next available field number — confirm against the file's actual current max before assigning
```
(No longer `optional` — every meetup requires a window now, ADR-016.)

`MeetupResponse`: same replacement for its own `scheduled_for_unix_seconds` field, plus `optional int64 closed_at_unix_seconds` (mirrors the existing `cancelled_at_unix_seconds` pattern).

`SubmitMeetupFeedbackRequest`: add `optional string notes = 7;`.

## Part D — gateway (`services/gateway/internal/handlers/meetups.go` + route table)

```go
mux.Handle("POST /v1/meetups/{id}/close", h.requireAuth(http.HandlerFunc(h.closeMeetup)))
```

Same shape as `cancelMeetup` — `middleware.UserIDFromContext(ctx)` passed as `host_user_id`, never trusted from the request body. Thread `window_start`/`window_end`/`closed_at`/`notes` through `meetupFromClient`/the request-building side for `createMeetup`/`submitMeetupFeedback`.

## Self-review checklist

- [ ] `LoginWithPassword` now pays the same cost (a real `verifyPassword` call) on every path — verified by reading the code, not just the diff.
- [ ] `CloseMeetup`'s authorization + precondition check happens in the single `UPDATE ... WHERE` (Part B) — no separate SELECT-then-UPDATE that could race.
- [ ] `window_end > window_start` checked both server-side (Part B's `CreateMeetup`) and DB-side (the `CHECK` constraint) — not just one.
- [ ] Migration backfill (Part B) actually runs before the `NOT NULL`/`DROP COLUMN` steps, in that order, so it doesn't fail against existing dev data.
- [ ] Rating eligibility (ADR-015's `SubmitRating`/`HasConfirmedHappened`) is untouched by any of this — confirm no accidental new dependency on `meetups.status` crept into that code path.

## Related

ADR-016 · ADR-015 · `backend/meetup-scheduling-PLAN.md` (the slice this evolves) · `frontend/meetup-lifecycle-PLAN.md`
