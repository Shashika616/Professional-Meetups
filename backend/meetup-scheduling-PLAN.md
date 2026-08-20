# Meetup Scheduling & Join Requests — Execution Plan (Backend)

Give this whole file to Claude Code as the task brief, same pattern as `backend/PLAN.md` and its Level 2/3 addendum. Built per `06 - Roadmap/Feature Build Plan Template.md` in the vault. Durable decisions behind this slice are recorded separately in ADR-013 (`04 - Decisions/ADR-013 - Host-Initiated Meetup Scheduling with Join Requests.md`) — read that first for the *why*; this file is disposable execution detail for the *how*.

**Sequencing: do not start until both of these are merged** — the Level 2/3 verification addendum (`frontend/PLAN.md`) and the session-refresh wiring fix (`frontend/PLAN.md`'s latest addendum). This slice's gateway routes reuse the JWT auth middleware and `trust_level` claim both of those depend on being stable; starting concurrently risks the same file-collision problem already flagged for those two.

## 1. Scope boundary

**In scope**: a new `meetup` service — create a Meetup (host, intent, timing, location, capacity), list/browse open Meetups by intent, request to join one, host accept/reject of individual requests, auto-reject on capacity overflow, the Safety Gate sub-flow (checklist acknowledgment, optional live-location toggle, check-in, post-meetup confirmation) for any Meetup with at least one accepted request, and push notifications (FCM) for request-created/accepted/rejected events.

**Explicitly not in scope**: the compatibility-matching engine (Phase 3, ranks/suggests Meetups — this slice's Meetups exist and are listable without it), real-time WebSocket delivery via the not-yet-built Realtime Gateway (push notifications + manual refresh cover this slice; swap in real-time later without changing this slice's data model), ride-sharing's extra verification stack (Meetup's `intent = ride_share` stays gated at trust level 4 by the same mechanism as everything else, but no driver/vehicle verification work here — that's ADR-004-deferred), SOS/emergency service (referenced by the safety checklist UI but not built here), in-app chat between host and requester (out of scope; acceptance surfaces contact only through whatever messaging exists today — flag if none does).

## 2. Prerequisites (human, not Claude Code)

1. **Firebase Cloud Messaging** — Shashika needs to: create/reuse a Firebase project layered on the existing GCP project (Firebase can attach to an existing GCP project without conflicting with anything already running there), enable Cloud Messaging, generate a service-account JSON key for server-side sends (Firebase Console → Project Settings → Service Accounts → Generate new private key), and provide it as `FIREBASE_SERVICE_ACCOUNT_JSON` (or a file path + `GOOGLE_APPLICATION_CREDENTIALS`, Claude Code's choice — match whatever pattern `LINKEDIN_CLIENT_SECRET` etc. already use for secret files vs. env vars in this repo). Keep the empty-credential-falls-back-to-logging pattern already established for Twilio/Resend — a `LoggingPushSender` that logs the notification payload instead of calling FCM when the credential is absent, so local dev/tests never need a real Firebase project.
2. This service needs no mapping/geocoding credential itself (Mapbox, per ADR-013 §4, switched from the originally-planned Google Maps — see `frontend/meetup-scheduling-PLAN.md`) — the frontend resolves an address to lat/lng client-side via Mapbox Search Box and sends the backend only the resolved coordinates + formatted address string to store. Confirm this division of responsibility before building; don't add a server-side geocoding call that duplicates it.
3. Confirm the backend stack still runs locally (`cd backend && docker compose up --build`) before adding a new service to it.

## 3. Build order

### Step A — Migration `0003_meetups.up.sql`

```sql
CREATE TYPE meetup_status AS ENUM ('open', 'full', 'cancelled', 'completed');
CREATE TYPE meetup_request_status AS ENUM ('pending', 'accepted', 'rejected', 'withdrawn');
CREATE TYPE intent_type AS ENUM ('coffee', 'lunch', 'networking', 'mentorship', 'ride_share', 'dating');

CREATE TABLE meetups (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    host_user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    intent          intent_type NOT NULL,
    -- NULL scheduled_for means "now/today" — created and immediately open,
    -- no future slot. Non-null is the "schedule for later" path.
    scheduled_for   TIMESTAMPTZ,
    location_lat    DOUBLE PRECISION NOT NULL,
    location_lng    DOUBLE PRECISION NOT NULL,
    -- Formatted address string from Places, display-only — never used for
    -- anything security-relevant server-side.
    location_label  TEXT NOT NULL,
    capacity        SMALLINT NOT NULL CHECK (capacity BETWEEN 1 AND 20),
    status          meetup_status NOT NULL DEFAULT 'open',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at    TIMESTAMPTZ
);
CREATE INDEX idx_meetups_intent_status ON meetups (intent, status) WHERE status = 'open';
CREATE INDEX idx_meetups_host ON meetups (host_user_id);

CREATE TABLE meetup_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meetup_id       UUID NOT NULL REFERENCES meetups(id) ON DELETE CASCADE,
    requester_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          meetup_request_status NOT NULL DEFAULT 'pending',
    -- Set when auto-rejected for capacity, distinct from a host's explicit
    -- rejection — the frontend shows different copy for each.
    auto_rejected   BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    -- One active request per user per meetup — re-requesting after
    -- withdrawing is allowed (no UNIQUE across all rows), but not two
    -- simultaneous pending/accepted requests from the same person.
    UNIQUE (meetup_id, requester_id, status) DEFERRABLE INITIALLY IMMEDIATE
);
CREATE INDEX idx_meetup_requests_meetup ON meetup_requests (meetup_id, status);
CREATE INDEX idx_meetup_requests_requester ON meetup_requests (requester_id);

-- Safety Gate state, one row per meetup once it has its first accepted
-- request. Kept as its own table rather than columns on meetups — this is
-- the "different retention/sensitivity class" pattern from
-- backend/ARCHITECTURE.md's data-sensitivity note: live-location sharing in
-- particular should be easy to purge/restrict independently of the meetup
-- record itself.
CREATE TABLE meetup_safety_state (
    meetup_id           UUID PRIMARY KEY REFERENCES meetups(id) ON DELETE CASCADE,
    checklist_ack_at    TIMESTAMPTZ,
    live_location_opt_in BOOLEAN NOT NULL DEFAULT false,
    checked_in_at       TIMESTAMPTZ,
    -- Per-participant post-meetup confirmation is a separate table (below),
    -- not a single boolean here — the flow needs it from *every* confirmed
    -- participant, not just the host.
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE meetup_feedback (
    meetup_id       UUID NOT NULL REFERENCES meetups(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    happened        BOOLEAN NOT NULL,
    felt_safe       BOOLEAN,
    profile_accurate BOOLEAN,
    would_meet_again BOOLEAN,
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (meetup_id, user_id)
);
```

Reuse `intent_type` as a real Postgres enum rather than free text — it's the server-side mirror of the frontend's `IntentType`; note the duplication explicitly in code comments (same pattern already used for pure-validator duplication) so future edits to one aren't forgotten on the other side.

### Step B — New service `services/meetup`, same shape as `services/auth`

`cmd/server`, `internal/{repository (+queries, +sqlcgen), service, events, notifications, config}`. Repository via `sqlc`, service layer holds business rules (capacity check, trust-level gate, status transitions), `events` package publishes to Pub/Sub (mirror `services/auth/internal/events`'s `Publisher` interface shape exactly — same `Logging`-style pattern isn't needed here since Pub/Sub already has a local emulator, unlike Twilio/Resend/FCM which are genuinely external).

**Trust-level gate, server-side, non-negotiable**: `CreateMeetup` and `RequestToJoin` both read the caller's `trust_level` from the JWT claim the gateway middleware already attaches (`middleware.UserIDFromContext` gives the ID; the service needs the trust level too — thread it through the same way, or re-fetch from `users` if the claim alone isn't sufficient trust for a write path — Claude Code's call, but the check must happen server-side regardless of what the client sends, same principle as every other trust decision in this codebase). Required-level table (mirrors `IntentType.requiredTrustLevel`, ADR-013): `ride_share`/`dating` → 4, everything else → 2. Reject with a 403-equivalent `apperror` if not met — do not silently downgrade or allow-with-a-warning.

**Capacity + auto-reject, as a DB transaction**: accepting a request must (a) check current accepted-count < capacity, (b) mark this request `accepted`, (c) if capacity is now reached, auto-reject every other still-`pending` request for that meetup and flip the meetup's `status` to `full` — all inside one transaction to avoid a race between two hosts... actually one host, but two near-simultaneous accept calls (double-tap, or accept + a capacity-triggered auto-reject racing) — use `SELECT ... FOR UPDATE` on the meetup row or an equivalent row lock, this is exactly the kind of concurrency bug a unit test can't catch and only a real-Postgres integration test will (Test strategy, below).

**Notifications package**: `internal/notifications`, an interface (`Sender` or similar) with `SendPushNotification(ctx, userID, title, body, data map[string]string) error`, a `LoggingPushSender` (default, logs instead of sending), and `FCMPushSender` (real, config-switched on `FIREBASE_SERVICE_ACCOUNT_JSON` presence — same pattern as `EmailSender`/`SmsSender`). Needs the recipient's FCM device token, which means: add a `device_tokens` table or a nullable `fcm_token` column on `users` (Claude Code's call on shape; a separate table is more consistent with this project's "separate table for things that change independently" pattern, since a user's registered device(s) can change without touching their identity record) and a new authenticated gateway route for the frontend to register/refresh its token. Flag this as a small necessary addition even though it's not explicitly in ADR-013's text — notifications can't be sent without it.

### Step C — Proto (`proto/meetup/v1/meetup.proto`), gRPC internal, gateway-fronted same as `auth`

RPCs: `CreateMeetup`, `ListOpenMeetups` (filter by intent, cursor-paginated, matches the frontend's existing `PagedResult` shape), `GetMeetup`, `ListMyMeetups` (hosted + requested, both), `RequestToJoin`, `WithdrawRequest`, `RespondToRequest` (accept/reject, host-only — verify `host_user_id` matches the caller, not just that *a* valid token was presented), `RegisterDeviceToken`, `AcknowledgeSafetyChecklist`, `SetLiveLocationOptIn`, `CheckIn`, `SubmitMeetupFeedback`, `CancelMeetup` (host-only, before any accepted request — cancelling a meetup with confirmed participants needs its own confirmation-and-notify path, don't let it silently vanish on people who already said yes).

### Step D — Gateway routes, `/v1/meetups/*`

All authenticated (reuse `middleware.Auth`, no new auth mechanism). Rate-limit `CreateMeetup`/`RequestToJoin` similarly to how `backend/PLAN.md`'s Level 2/3 addendum rate-limited verification starts — a spam-created-meetups or spam-join-requests vector is the same shape of abuse as spam-OTP-sends.

### Step E — Publish events, dispatch notifications

`meetup.request.created`, `meetup.request.accepted`, `meetup.request.rejected` (include `auto_rejected` reason) published to Pub/Sub from the service layer on each transition. A small consumer (own goroutine/worker inside `services/meetup`, or a dedicated `notification-dispatcher` — Claude Code's call, but don't make it a whole separate Cloud Run service for this volume) subscribes and calls `notifications.Sender` to push to the relevant user(s). Add the new topics to `docker-compose.yml`'s `pubsub-setup` step, same `curl PUT` pattern already there for `user-onboarded`.

## 4. Test strategy

- **Unit**: trust-level gate logic (per-intent table, both allow and reject cases), capacity-check logic, status-transition rules (can't accept an already-resolved request, can't join your own meetup, can't double-request while a request is already pending).
- **Component (fake FCM)**: `FCMPushSender` against a stub HTTP server, confirming payload shape and that a send failure doesn't roll back the underlying DB transaction that triggered it (a failed push shouldn't un-accept a request).
- **Integration (real Postgres)**: the capacity-race scenario above — two concurrent `RespondToRequest` accept calls against a capacity-1 meetup with two pending requests; exactly one succeeds, the other is rejected with a clear "meetup full" reason, no double-accept.
- **Security-relevant**: a non-host calling `RespondToRequest` on someone else's meetup is rejected; a Level 1 user calling `CreateMeetup`/`RequestToJoin` on a Level-2-gated intent is rejected; `CancelMeetup` by a non-host is rejected.

## 5. CI update

New service needs: its own `Dockerfile` (same multi-stage distroless pattern as `services/auth`/`services/gateway`), addition to `docker-compose.yml` (new `meetup` service block, new topics in `pubsub-setup`, migration already covered by the existing `migrate` service picking up `0003_*`), and addition to `backend-ci.yml`'s build/test matrix once that workflow exists (it's still only planned per `docs/03-architecture/system-architecture.md` — if it still doesn't exist when this lands, note that gap again rather than silently building CI for one service only).

## 6. Self-review checklist

- [ ] Trust-level check happens server-side, reads the JWT claim (or a fresh DB read), never trusts a client-supplied trust level.
- [ ] The capacity-race integration test actually exercises real concurrent requests against real Postgres, not two sequential calls that happen to be near each other in test code.
- [ ] A host cannot accept/reject requests on a meetup they don't own; a requester cannot withdraw someone else's request.
- [ ] `LoggingPushSender` is the default with no Firebase credential present — `docker compose up --build` works with an empty `FIREBASE_SERVICE_ACCOUNT_JSON`.
- [ ] No raw device token, live-location coordinate stream, or feedback content is logged or exposed in an error message.
- [ ] `go vet`/`golangci-lint` clean.
- [ ] `docker compose up --build` from a clean checkout (`docker compose down -v` first).
- [ ] Bring the diff back to Cowork for review before merging.

## 7. Explicitly not in this slice

Compatibility-matching/ranking engine, real-time WebSocket delivery (Realtime Gateway), ride-share driver/vehicle verification, SOS/emergency backend, in-app chat, a waitlist (capacity overflow is auto-reject only, per ADR-013), refunds/payments (no monetization touches this slice).
