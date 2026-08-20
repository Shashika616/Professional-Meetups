package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/professional-connections/backend/services/meetup/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
)

// fakeMeetupRepository is an in-memory stand-in for repository.MeetupRepository.
type fakeMeetupRepository struct {
	mu        sync.Mutex
	byID      map[string]repository.Meetup
	nextID    int
	getErr    error
	createErr error
}

func newFakeMeetupRepository() *fakeMeetupRepository {
	return &fakeMeetupRepository{byID: map[string]repository.Meetup{}}
}

func (f *fakeMeetupRepository) Create(_ context.Context, m repository.NewMeetup) (repository.Meetup, error) {
	if f.createErr != nil {
		return repository.Meetup{}, f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	created := repository.Meetup{
		ID:            fmt.Sprintf("meetup-%d", f.nextID),
		HostUserID:    m.HostUserID,
		Intent:        m.Intent,
		WindowStart:   m.WindowStart,
		WindowEnd:     m.WindowEnd,
		LocationLat:   m.LocationLat,
		LocationLng:   m.LocationLng,
		LocationLabel: m.LocationLabel,
		Capacity:      m.Capacity,
		Status:        repository.MeetupStatusOpen,
		CreatedAt:     time.Now(),
	}
	f.byID[created.ID] = created
	return created, nil
}

func (f *fakeMeetupRepository) GetByID(_ context.Context, id, _ string) (repository.Meetup, error) {
	if f.getErr != nil {
		return repository.Meetup{}, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return repository.Meetup{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return m, nil
}

func (f *fakeMeetupRepository) ListOpen(context.Context, repository.Intent, string, *repository.Cursor, int) ([]repository.Meetup, *repository.Cursor, error) {
	return nil, nil, nil
}

func (f *fakeMeetupRepository) ListByHost(context.Context, string) ([]repository.Meetup, error) {
	return nil, nil
}

func (f *fakeMeetupRepository) ListRequestedByUser(context.Context, string) ([]repository.Meetup, error) {
	return nil, nil
}

func (f *fakeMeetupRepository) Cancel(_ context.Context, id string) (repository.Meetup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok {
		return repository.Meetup{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	m.Status = repository.MeetupStatusCancelled
	f.byID[id] = m
	return m, nil
}

// Close mirrors CloseMeetup's real SQL WHERE clause's four conditions
// (right meetup, right host, currently open-ish, window started) — zero
// match maps to apperror.ErrNotFound either way, same as the real query
// returning zero rows, so CloseMeetup's re-fetch-to-distinguish-why logic
// can be exercised against this fake.
func (f *fakeMeetupRepository) Close(_ context.Context, id, hostUserID string) (repository.Meetup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.byID[id]
	if !ok || m.HostUserID != hostUserID ||
		(m.Status != repository.MeetupStatusOpen && m.Status != repository.MeetupStatusFull) ||
		time.Now().Before(m.WindowStart) {
		return repository.Meetup{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	m.Status = repository.MeetupStatusCompleted
	now := time.Now()
	m.ClosedAt = &now
	f.byID[id] = m
	return m, nil
}

func (f *fakeMeetupRepository) put(m repository.Meetup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[m.ID] = m
}

// fakeMeetupRequestRepository is an in-memory stand-in for
// repository.MeetupRequestRepository.
type fakeMeetupRequestRepository struct {
	mu        sync.Mutex
	byID      map[string]repository.MeetupRequest
	nextID    int
	createErr error
	acceptErr error
}

func newFakeMeetupRequestRepository() *fakeMeetupRequestRepository {
	return &fakeMeetupRequestRepository{byID: map[string]repository.MeetupRequest{}}
}

func (f *fakeMeetupRequestRepository) Create(_ context.Context, meetupID, requesterID string) (repository.MeetupRequest, error) {
	if f.createErr != nil {
		return repository.MeetupRequest{}, f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.byID {
		if r.MeetupID == meetupID && r.RequesterID == requesterID && (r.Status == repository.RequestStatusPending || r.Status == repository.RequestStatusAccepted) {
			return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrConflict)
		}
	}
	f.nextID++
	// Deliberately leaves RequesterFullName/RequesterProfilePhotoURL/
	// RequesterTrustLevel blank, matching the real Postgres repository's
	// Create (no join against users — meetup_requests_postgres.go's
	// meetupRequestFromRow doc comment). A test that reads a request's
	// display info straight off Create's return value, rather than calling
	// GetByID for the joined view, would fail against the fake the same
	// way it would against the real thing — this is what caught
	// RequestToJoin's "wants to join" notification once using an empty
	// name (fixed in requests.go).
	created := repository.MeetupRequest{
		ID:          fmt.Sprintf("request-%d", f.nextID),
		MeetupID:    meetupID,
		RequesterID: requesterID,
		Status:      repository.RequestStatusPending,
		CreatedAt:   time.Now(),
	}
	f.byID[created.ID] = created
	return created, nil
}

// GetByID simulates the real repository's join against users — unlike
// Create, it always returns display info populated, even though the
// underlying stored row (see Create above) has none.
func (f *fakeMeetupRequestRepository) GetByID(_ context.Context, id string) (repository.MeetupRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[id]
	if !ok {
		return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	r.RequesterFullName = "Test Requester"
	return r, nil
}

// ListForMeetup also simulates the join, same as GetByID.
func (f *fakeMeetupRequestRepository) ListForMeetup(_ context.Context, meetupID string) ([]repository.MeetupRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []repository.MeetupRequest
	for _, r := range f.byID {
		if r.MeetupID == meetupID {
			r.RequesterFullName = "Test Requester"
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeMeetupRequestRepository) Withdraw(_ context.Context, id string) (repository.MeetupRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[id]
	if !ok {
		return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	if r.Status != repository.RequestStatusPending {
		return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrConflict)
	}
	r.Status = repository.RequestStatusWithdrawn
	f.byID[id] = r
	return r, nil
}

func (f *fakeMeetupRequestRepository) Reject(_ context.Context, id string) (repository.MeetupRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[id]
	if !ok {
		return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	if r.Status != repository.RequestStatusPending {
		return repository.MeetupRequest{}, fmt.Errorf("fake: %w", apperror.ErrConflict)
	}
	r.Status = repository.RequestStatusRejected
	f.byID[id] = r
	return r, nil
}

func (f *fakeMeetupRequestRepository) Accept(_ context.Context, id string) (repository.MeetupRequest, bool, []repository.MeetupRequest, error) {
	if f.acceptErr != nil {
		return repository.MeetupRequest{}, false, nil, f.acceptErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[id]
	if !ok {
		return repository.MeetupRequest{}, false, nil, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	if r.Status != repository.RequestStatusPending {
		return repository.MeetupRequest{}, false, nil, fmt.Errorf("fake: %w", apperror.ErrConflict)
	}
	r.Status = repository.RequestStatusAccepted
	f.byID[id] = r
	return r, false, nil, nil
}

func (f *fakeMeetupRequestRepository) put(r repository.MeetupRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[r.ID] = r
}

// fakeSafetyStateRepository is an in-memory stand-in for
// repository.SafetyStateRepository.
type fakeSafetyStateRepository struct {
	mu   sync.Mutex
	byID map[string]repository.SafetyState
}

func newFakeSafetyStateRepository() *fakeSafetyStateRepository {
	return &fakeSafetyStateRepository{byID: map[string]repository.SafetyState{}}
}

func (f *fakeSafetyStateRepository) EnsureExists(_ context.Context, meetupID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[meetupID]; !ok {
		f.byID[meetupID] = repository.SafetyState{MeetupID: meetupID}
	}
	return nil
}

func (f *fakeSafetyStateRepository) Get(_ context.Context, meetupID string) (repository.SafetyState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[meetupID]
	if !ok {
		return repository.SafetyState{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return s, nil
}

func (f *fakeSafetyStateRepository) AcknowledgeChecklist(_ context.Context, meetupID string) (repository.SafetyState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[meetupID]
	if !ok {
		return repository.SafetyState{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	now := time.Now()
	s.ChecklistAckAt = &now
	f.byID[meetupID] = s
	return s, nil
}

func (f *fakeSafetyStateRepository) SetLiveLocationOptIn(_ context.Context, meetupID string, optIn bool) (repository.SafetyState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[meetupID]
	if !ok {
		return repository.SafetyState{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	s.LiveLocationOptIn = optIn
	f.byID[meetupID] = s
	return s, nil
}

func (f *fakeSafetyStateRepository) CheckIn(_ context.Context, meetupID string) (repository.SafetyState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[meetupID]
	if !ok {
		return repository.SafetyState{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	now := time.Now()
	s.CheckedInAt = &now
	f.byID[meetupID] = s
	return s, nil
}

// fakeFeedbackRepository is an in-memory stand-in for
// repository.FeedbackRepository.
type fakeFeedbackRepository struct {
	mu    sync.Mutex
	calls []fakeFeedbackCall
}

type fakeFeedbackCall struct {
	MeetupID, UserID                          string
	Happened                                  bool
	FeltSafe, ProfileAccurate, WouldMeetAgain *bool
	Notes                                     *string
}

func newFakeFeedbackRepository() *fakeFeedbackRepository {
	return &fakeFeedbackRepository{}
}

func (f *fakeFeedbackRepository) Upsert(_ context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeFeedbackCall{meetupID, userID, happened, feltSafe, profileAccurate, wouldMeetAgain, notes})
	return nil
}

// fakeRatingRepository is an in-memory stand-in for
// repository.RatingRepository. participants/confirmed are populated by
// tests via setParticipant/setConfirmed before exercising SubmitRating's
// guard conditions; submitted records every accepted Submit call so tests
// can assert on it, and submitErr lets a test force Submit's own error path
// (e.g. simulating the DB's duplicate/self-rating rejection) independently
// of the service-layer guards.
type fakeRatingRepository struct {
	mu           sync.Mutex
	participants map[string]bool // "meetupID/userID" -> is a participant
	confirmed    map[string]bool // "meetupID/userID" -> confirmed happened=true
	submitted    []fakeRatingCall
	submitErr    error
}

type fakeRatingCall struct {
	MeetupID, RaterID, RatedID string
	Score                      int
}

func newFakeRatingRepository() *fakeRatingRepository {
	return &fakeRatingRepository{participants: map[string]bool{}, confirmed: map[string]bool{}}
}

func (f *fakeRatingRepository) setParticipant(meetupID, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.participants[meetupID+"/"+userID] = true
}

func (f *fakeRatingRepository) setConfirmed(meetupID, userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirmed[meetupID+"/"+userID] = true
}

func (f *fakeRatingRepository) IsParticipant(_ context.Context, meetupID, userID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.participants[meetupID+"/"+userID], nil
}

func (f *fakeRatingRepository) HasConfirmedHappened(_ context.Context, meetupID, userID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.confirmed[meetupID+"/"+userID], nil
}

func (f *fakeRatingRepository) ListRatable(context.Context, string, string) ([]repository.RatableParticipant, error) {
	return nil, nil
}

func (f *fakeRatingRepository) Submit(_ context.Context, meetupID, raterID, ratedID string, score int) error {
	if f.submitErr != nil {
		return f.submitErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submitted = append(f.submitted, fakeRatingCall{meetupID, raterID, ratedID, score})
	return nil
}

// fakeDeviceTokenRepository is an in-memory stand-in for
// repository.DeviceTokenRepository.
type fakeDeviceTokenRepository struct {
	mu     sync.Mutex
	tokens map[string][]string
}

func newFakeDeviceTokenRepository() *fakeDeviceTokenRepository {
	return &fakeDeviceTokenRepository{tokens: map[string][]string{}}
}

func (f *fakeDeviceTokenRepository) Upsert(_ context.Context, userID, fcmToken string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[userID] = append(f.tokens[userID], fcmToken)
	return nil
}

func (f *fakeDeviceTokenRepository) ListForUser(_ context.Context, userID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokens[userID], nil
}

// fakePublisher is an in-memory stand-in for events.Publisher.
type fakePublisher struct {
	mu                     sync.Mutex
	createdCalls           int
	acceptedCalls          int
	rejectedCalls          int
	lastRejectedAutoReject bool
}

func (f *fakePublisher) PublishRequestCreated(context.Context, string, string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createdCalls++
	return nil
}

func (f *fakePublisher) PublishRequestAccepted(context.Context, string, string, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptedCalls++
	return nil
}

func (f *fakePublisher) PublishRequestRejected(_ context.Context, _, _, _, _ string, autoRejected bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectedCalls++
	f.lastRejectedAutoReject = autoRejected
	return nil
}

func (f *fakePublisher) Close() error { return nil }

// fakeNotificationSender is an in-memory stand-in for notifications.Sender.
type fakeNotificationSender struct {
	mu       sync.Mutex
	calls    int
	err      error
	lastBody string
}

func (f *fakeNotificationSender) SendPushNotification(_ context.Context, _, _, body string, _ map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastBody = body
	return f.err
}
