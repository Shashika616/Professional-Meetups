// Package repository is the persistence boundary for services/meetup:
// interfaces first, Postgres implementation second — same pattern as
// services/auth/internal/repository. internal/service/ depends on these
// interfaces, never directly on Postgres or sqlcgen.
package repository

import (
	"context"
	"time"
)

// Intent mirrors the intent_type Postgres enum (db/migrations/0003) and the
// frontend's IntentType (frontend/lib/core/models/intent_type.dart) — three-
// way duplication, noted explicitly (ADR-013, backend/meetup-scheduling-
// PLAN.md Step A).
type Intent string

const (
	IntentCoffee     Intent = "coffee"
	IntentLunch      Intent = "lunch"
	IntentNetworking Intent = "networking"
	IntentMentorship Intent = "mentorship"
	IntentRideShare  Intent = "ride_share"
	IntentDating     Intent = "dating"
)

type MeetupStatus string

const (
	MeetupStatusOpen      MeetupStatus = "open"
	MeetupStatusFull      MeetupStatus = "full"
	MeetupStatusCancelled MeetupStatus = "cancelled"
	MeetupStatusCompleted MeetupStatus = "completed"
)

type MeetupRequestStatus string

const (
	RequestStatusPending   MeetupRequestStatus = "pending"
	RequestStatusAccepted  MeetupRequestStatus = "accepted"
	RequestStatusRejected  MeetupRequestStatus = "rejected"
	RequestStatusWithdrawn MeetupRequestStatus = "withdrawn"
)

// Meetup is a scheduled meetup, including host display info (name/photo/
// trust level/rating) read directly from the shared users table. This
// service reads several public-safe display columns off users this way; as
// of ADR-015 it also *writes* two of them (rating_average/rating_count,
// via RatingRepository.Submit) — the one exception to the otherwise
// read-only boundary, justified there by services/auth and services/meetup
// sharing one literal Postgres database (no Pub/Sub needed for something
// already this local).
type Meetup struct {
	ID                  string
	HostUserID          string
	HostFullName        string
	HostProfilePhotoURL string
	HostTrustLevel      int
	HostRatingAverage   float64
	HostRatingCount     int
	Intent              Intent
	// WindowStart/WindowEnd replace the old nullable ScheduledFor (ADR-016)
	// — every meetup, "today" included, now requires a real time range,
	// both NOT NULL, DB-enforced WindowEnd > WindowStart.
	WindowStart   time.Time
	WindowEnd     time.Time
	LocationLat   float64
	LocationLng   float64
	LocationLabel string
	Capacity      int
	AcceptedCount int
	Status        MeetupStatus
	CreatedAt     time.Time
	CancelledAt   *time.Time
	// ClosedAt is set by CloseMeetup (ADR-016) — nil until a host closes the
	// meetup, mirrors CancelledAt's shape.
	ClosedAt *time.Time
	// MyRequestStatus is only populated by viewer-aware queries (GetByID,
	// ListOpen) — nil there means the viewer never requested to join. Always
	// nil from ListByHost (a host viewing their own meetups never has a
	// request on them) — callers must not read it from that path.
	MyRequestStatus *MeetupRequestStatus
	// MyRequestAutoRejected is only meaningful when MyRequestStatus is
	// RequestStatusRejected, and only populated by ListRequestedByUser (the
	// "My Meetups" requested list) — that's the one place a requester needs
	// to distinguish the host's explicit rejection from a system auto-reject
	// (capacity filled before the host acted). False elsewhere.
	MyRequestAutoRejected bool
}

// NewMeetup is the input to MeetupRepository.Create.
type NewMeetup struct {
	HostUserID    string
	Intent        Intent
	WindowStart   time.Time
	WindowEnd     time.Time
	LocationLat   float64
	LocationLng   float64
	LocationLabel string
	Capacity      int
}

// Cursor is an opaque (to callers outside this package) keyset-pagination
// position — the (created_at, id) of the last row of the previous page.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// MeetupRepository is the persistence boundary for meetup records.
type MeetupRepository interface {
	Create(ctx context.Context, m NewMeetup) (Meetup, error)
	// GetByID returns apperror.ErrNotFound (wrapped) if id doesn't exist.
	// viewerID populates MyRequestStatus relative to that specific caller.
	GetByID(ctx context.Context, id, viewerID string) (Meetup, error)
	// ListOpen returns open meetups for intent, newest first, cursor-
	// paginated. cursor nil means the first page. Returns one page of at
	// most pageSize meetups and the cursor for the next page (nil if this
	// was the last page).
	ListOpen(ctx context.Context, intent Intent, viewerID string, cursor *Cursor, pageSize int) ([]Meetup, *Cursor, error)
	// ListByHost returns every meetup hostID hosts, newest first.
	// MyRequestStatus is always nil on these rows (see Meetup's doc comment).
	ListByHost(ctx context.Context, hostID string) ([]Meetup, error)
	// ListRequestedByUser returns one row per meetup userID has an active or
	// historical request on, newest-meetup first, MyRequestStatus always
	// populated (the requester's latest request status for that meetup).
	ListRequestedByUser(ctx context.Context, userID string) ([]Meetup, error)
	// Cancel returns apperror.ErrNotFound (wrapped) if id doesn't exist.
	// Host-ownership and "no accepted requests yet" are checked by the
	// service layer before calling this, not here.
	Cancel(ctx context.Context, id string) (Meetup, error)
	// Close transitions the meetup to 'completed' — the entire
	// authorization/precondition check (right host, currently open-ish,
	// window started) is the query's own WHERE clause, not a separate
	// SELECT-then-UPDATE (ADR-016). Returns apperror.ErrNotFound (wrapped)
	// if zero rows matched; the service layer re-fetches to distinguish
	// *why* (wrong host / already closed / window not started) for a
	// useful error message.
	Close(ctx context.Context, id, hostUserID string) (Meetup, error)
}

// MeetupRequest is another user's request to join a Meetup, including
// requester display info read from users the same way Meetup carries host
// display info.
type MeetupRequest struct {
	ID                       string
	MeetupID                 string
	RequesterID              string
	RequesterFullName        string
	RequesterProfilePhotoURL string
	RequesterTrustLevel      int
	RequesterRatingAverage   float64
	RequesterRatingCount     int
	Status                   MeetupRequestStatus
	AutoRejected             bool
	CreatedAt                time.Time
	ResolvedAt               *time.Time
}

// MeetupRequestRepository is the persistence boundary for join requests.
type MeetupRequestRepository interface {
	// Create returns apperror.ErrConflict (wrapped) if requesterID already
	// has a pending or accepted request on meetupID — migration 0003's
	// UNIQUE(meetup_id, requester_id, status) constraint is what actually
	// enforces this; this method catches the resulting 23505 rather than
	// pre-checking (same pattern as users_postgres.go's phone/email
	// conflict handling).
	Create(ctx context.Context, meetupID, requesterID string) (MeetupRequest, error)
	// GetByID returns apperror.ErrNotFound (wrapped) if id doesn't exist.
	GetByID(ctx context.Context, id string) (MeetupRequest, error)
	// ListForMeetup returns every request (any status) on meetupID, oldest
	// first — the host's request-management view.
	ListForMeetup(ctx context.Context, meetupID string) ([]MeetupRequest, error)
	// Withdraw returns apperror.ErrConflict (wrapped) if the request is not
	// currently pending (already resolved, or already withdrawn).
	Withdraw(ctx context.Context, id string) (MeetupRequest, error)
	// Reject is the host's explicit rejection — distinct from the
	// capacity-triggered auto-reject inside Accept. Returns
	// apperror.ErrConflict (wrapped) if the request is not currently
	// pending.
	Reject(ctx context.Context, id string) (MeetupRequest, error)
	// Accept runs the capacity check and, if this acceptance fills the
	// meetup, the auto-reject-everyone-else transition — all inside one DB
	// transaction with a row lock on the meetup (backend/meetup-scheduling-
	// PLAN.md Step B): two near-simultaneous accept calls against the same
	// meetup can never both succeed past capacity. Returns
	// apperror.ErrConflict (wrapped) if the request is not pending or the
	// meetup is not open (already full/cancelled/completed — a defensive
	// re-check under the lock, not just trusting an earlier read).
	// meetupNowFull is true iff this acceptance was the one that reached
	// capacity; autoRejected lists every other request that was rejected as
	// a side effect, for the caller to notify.
	Accept(ctx context.Context, id string) (accepted MeetupRequest, meetupNowFull bool, autoRejected []MeetupRequest, err error)
}

// SafetyState is a meetup's Safety Gate progress (ADR-013 § 3).
type SafetyState struct {
	MeetupID          string
	ChecklistAckAt    *time.Time
	LiveLocationOptIn bool
	CheckedInAt       *time.Time
}

// SafetyStateRepository is the persistence boundary for Safety Gate state.
type SafetyStateRepository interface {
	// EnsureExists creates the row if it doesn't already exist — idempotent,
	// safe to call every time a meetup gets a newly-accepted request.
	EnsureExists(ctx context.Context, meetupID string) error
	// Get returns apperror.ErrNotFound (wrapped) if no row exists yet (the
	// meetup has no accepted requests, so the Safety Gate hasn't started).
	Get(ctx context.Context, meetupID string) (SafetyState, error)
	AcknowledgeChecklist(ctx context.Context, meetupID string) (SafetyState, error)
	SetLiveLocationOptIn(ctx context.Context, meetupID string, optIn bool) (SafetyState, error)
	CheckIn(ctx context.Context, meetupID string) (SafetyState, error)
}

// FeedbackRepository is the persistence boundary for post-meetup feedback
// (Safety UX Flows.md's five questions — felt_safe/profile_accurate/
// would_meet_again are nil when happened is false, there's nothing
// meaningful to ask if the meetup didn't happen).
type FeedbackRepository interface {
	Upsert(ctx context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string) error
}

// RatableParticipant is another participant of a meetup the viewer can
// (or already did) rate — see RatingRepository.ListRatable.
type RatableParticipant struct {
	UserID          string
	FullName        string
	ProfilePhotoURL string
	TrustLevel      int
	AlreadyRated    bool
}

// RatingRepository is the persistence boundary for post-meetup star ratings
// (ADR-015, docs/02-domain/domain-model.md § Rating). Authorization
// (participant checks, the rater's confirmed-attendance gate) is enforced
// by the service layer before calling Submit, same division of
// responsibility as MeetupRequestRepository — this layer only encodes the
// invariants a DB constraint can't be trusted to explain on its own
// (self-rating, duplicate submission) via the error mapping documented on
// Submit itself.
type RatingRepository interface {
	// IsParticipant reports whether userID is the host or an accepted
	// requester of meetupID — the "ratable set."
	IsParticipant(ctx context.Context, meetupID, userID string) (bool, error)
	// HasConfirmedHappened reports whether userID has a meetup_feedback row
	// for meetupID with happened=true — the rating-eligibility gate,
	// checked against the *rater* only (ADR-015).
	HasConfirmedHappened(ctx context.Context, meetupID, userID string) (bool, error)
	// ListRatable returns meetupID's other participants (host + accepted
	// requesters, excluding viewerID), each flagged with whether viewerID
	// already rated them.
	ListRatable(ctx context.Context, meetupID, viewerID string) ([]RatableParticipant, error)
	// Submit records raterID's score for ratedID on meetupID and
	// recomputes ratedID's users.rating_average/rating_count in the same
	// transaction. Returns apperror.ErrConflict (wrapped) on a duplicate
	// (meetup_id, rater_user_id, rated_user_id) submission, and
	// apperror.ErrInvalidInput (wrapped) if the DB's
	// CHECK(rater_user_id <> rated_user_id) fires (a malformed direct API
	// call — the UI never offers self as ratable).
	Submit(ctx context.Context, meetupID, raterID, ratedID string, score int) error
}

// DeviceTokenRepository is the persistence boundary for FCM push tokens.
type DeviceTokenRepository interface {
	// Upsert reassigns fcmToken to userID if it was previously registered
	// under a different account (shared device, account switch) — see
	// migration 0003's comment on device_tokens.
	Upsert(ctx context.Context, userID, fcmToken string) error
	ListForUser(ctx context.Context, userID string) ([]string, error)
}
