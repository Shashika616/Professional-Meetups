-- Host-initiated meetup scheduling with join requests (ADR-013). See
-- backend/meetup-scheduling-PLAN.md Step A for the design rationale behind
-- each table.

CREATE TYPE meetup_status AS ENUM ('open', 'full', 'cancelled', 'completed');
CREATE TYPE meetup_request_status AS ENUM ('pending', 'accepted', 'rejected', 'withdrawn');

-- Server-side mirror of the frontend's IntentType (frontend/lib/core/models/
-- intent_type.dart) — a change to one side (adding/renaming an intent) must
-- be made on the other too, same explicit-duplication-note pattern already
-- used for validators.dart vs. the backend's OTP domain-rejection lists.
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
    -- Formatted address string from Mapbox Search Box, display-only — never
    -- used for anything security-relevant server-side (ADR-013 § 4).
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
    meetup_id            UUID PRIMARY KEY REFERENCES meetups(id) ON DELETE CASCADE,
    checklist_ack_at     TIMESTAMPTZ,
    live_location_opt_in BOOLEAN NOT NULL DEFAULT false,
    checked_in_at        TIMESTAMPTZ,
    -- Per-participant post-meetup confirmation is a separate table (below),
    -- not a single boolean here — the flow needs it from *every* confirmed
    -- participant, not just the host.
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE meetup_feedback (
    meetup_id         UUID NOT NULL REFERENCES meetups(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    happened          BOOLEAN NOT NULL,
    felt_safe         BOOLEAN,
    profile_accurate  BOOLEAN,
    would_meet_again  BOOLEAN,
    submitted_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (meetup_id, user_id)
);

-- Not in ADR-013's text directly, but push notifications (§ 6) can't be
-- sent without somewhere to register a recipient device — flagged in
-- backend/meetup-scheduling-PLAN.md Step B as a small necessary addition.
-- A separate table (not a column on users) since a user's registered
-- device(s) change independently of their identity record, same reasoning
-- as meetup_safety_state above. UNIQUE on fcm_token, not a composite key —
-- a token identifies one physical device install; if it's later registered
-- under a different account (shared device, account switch), upserting by
-- token reassigns ownership rather than leaving a stale row pointing at the
-- previous account.
CREATE TABLE device_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fcm_token   TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_device_tokens_user_id ON device_tokens (user_id);

CREATE TRIGGER device_tokens_set_updated_at
    BEFORE UPDATE ON device_tokens
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER meetup_safety_state_set_updated_at
    BEFORE UPDATE ON meetup_safety_state
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
