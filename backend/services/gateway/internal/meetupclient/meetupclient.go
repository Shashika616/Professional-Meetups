// Package meetupclient wraps the generated gRPC client for the meetup
// service. Handlers call this package's typed methods, never the generated
// stub directly — mirrors internal/authclient's shape exactly.
package meetupclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	meetupv1 "github.com/professional-connections/backend/shared/proto/meetup/v1"
)

// connectTimeout bounds how long New waits for the initial connection to
// the meetup service before failing fast — a service that can't reach its
// dependencies should crash at startup, not accept traffic and fail every
// request.
const connectTimeout = 5 * time.Second

// Meetup is this package's own representation of a meetup, decoupled from
// the generated protobuf type — mirrors authclient.Session/Profile's
// pattern. Intent/Status/MyRequestStatus are plain lowercase strings
// (matching the Postgres enum values, e.g. "coffee", "open", "pending") —
// converted once here, not carried as proto enums into the JSON layer.
type Meetup struct {
	ID                     string
	HostUserID             string
	HostFullName           string
	HostProfilePhotoURL    string
	HostTrustLevel         int32
	HostRatingAverage      float64
	HostRatingCount        int32
	Intent                 string
	WindowStartUnixSeconds int64
	WindowEndUnixSeconds   int64
	LocationLat            float64
	LocationLng            float64
	LocationLabel          string
	Capacity               int32
	AcceptedCount          int32
	Status                 string
	CreatedAtUnixSeconds   int64
	CancelledAtUnixSeconds *int64
	ClosedAtUnixSeconds    *int64
	IsHostedByMe           bool
	MyRequestStatus        *string
	MyRequestAutoRejected  bool
}

// MeetupRequest is this package's own representation of a join request.
type MeetupRequest struct {
	ID                       string
	MeetupID                 string
	RequesterID              string
	RequesterFullName        string
	RequesterProfilePhotoURL string
	RequesterTrustLevel      int32
	RequesterRatingAverage   float64
	RequesterRatingCount     int32
	Status                   string
	AutoRejected             bool
	CreatedAtUnixSeconds     int64
	ResolvedAtUnixSeconds    *int64
}

// RatableParticipant is another participant of a meetup the viewer can (or
// already did) rate — see Client.ListRatableParticipants (ADR-015).
type RatableParticipant struct {
	UserID          string
	FullName        string
	ProfilePhotoURL string
	TrustLevel      int32
	AlreadyRated    bool
}

// SafetyState is this package's own representation of Safety Gate progress.
type SafetyState struct {
	MeetupID                  string
	ChecklistAckAtUnixSeconds *int64
	LiveLocationOptIn         bool
	CheckedInAtUnixSeconds    *int64
}

// Client is the gateway's view of the meetup service. Every method that
// needs the caller's identity takes user_id (and trust_level, where the
// RPC needs it) explicitly, set by the caller (internal/handlers) from the
// verified JWT — never a client-supplied value, same discipline as
// authclient.
type Client interface {
	CreateMeetup(ctx context.Context, hostUserID string, hostTrustLevel int32, intent string, windowStartUnixSeconds, windowEndUnixSeconds int64, lat, lng float64, locationLabel string, capacity int32) (Meetup, error)
	ListOpenMeetups(ctx context.Context, userID, intent, cursor string, pageSize int32) (meetups []Meetup, nextCursor string, err error)
	GetMeetup(ctx context.Context, meetupID, userID string) (Meetup, error)
	ListMyMeetups(ctx context.Context, userID string) (hosted, requested []Meetup, err error)
	ListMeetupRequests(ctx context.Context, meetupID, hostUserID string) ([]MeetupRequest, error)
	RequestToJoin(ctx context.Context, meetupID, requesterID string, requesterTrustLevel int32) (MeetupRequest, error)
	WithdrawRequest(ctx context.Context, requestID, requesterID string) error
	RespondToRequest(ctx context.Context, requestID, hostUserID string, accept bool) (MeetupRequest, error)
	RegisterDeviceToken(ctx context.Context, userID, fcmToken string) error
	GetSafetyState(ctx context.Context, meetupID string) (SafetyState, error)
	AcknowledgeSafetyChecklist(ctx context.Context, meetupID, userID string) (SafetyState, error)
	SetLiveLocationOptIn(ctx context.Context, meetupID, userID string, optIn bool) (SafetyState, error)
	CheckIn(ctx context.Context, meetupID, userID string) (SafetyState, error)
	SubmitMeetupFeedback(ctx context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string) error
	ListRatableParticipants(ctx context.Context, meetupID, viewerID string) ([]RatableParticipant, error)
	SubmitRating(ctx context.Context, meetupID, raterUserID, ratedUserID string, score int32) error
	// CloseMeetup is host-only — "meetup is done" (ADR-016), reviving the
	// previously-unused COMPLETED status. Independent of rating
	// eligibility (ADR-015's meetup_feedback.happened gate is unaffected).
	CloseMeetup(ctx context.Context, meetupID, hostUserID string) (Meetup, error)
	CancelMeetup(ctx context.Context, meetupID, hostUserID string) error

	Close() error
}

type grpcClient struct {
	conn   *grpc.ClientConn
	client meetupv1.MeetupServiceClient
}

// New connects to the meetup service at addr (e.g. "meetup:9091"), blocking
// until the connection is ready or connectTimeout elapses.
func New(addr string) (Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("meetupclient: create grpc client for %s: %w", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, fmt.Errorf("meetupclient: connect to meetup service at %s: %w", addr, ctx.Err())
		}
	}

	return &grpcClient{conn: conn, client: meetupv1.NewMeetupServiceClient(conn)}, nil
}

func (c *grpcClient) Close() error {
	return c.conn.Close()
}

func (c *grpcClient) CreateMeetup(
	ctx context.Context, hostUserID string, hostTrustLevel int32, intent string,
	windowStartUnixSeconds, windowEndUnixSeconds int64, lat, lng float64, locationLabel string, capacity int32,
) (Meetup, error) {
	protoIntent, ok := intentToProto[intent]
	if !ok {
		return Meetup{}, fmt.Errorf("meetupclient: unrecognized intent %q", intent)
	}
	resp, err := c.client.CreateMeetup(ctx, &meetupv1.CreateMeetupRequest{
		HostUserId:             hostUserID,
		HostTrustLevel:         hostTrustLevel,
		Intent:                 protoIntent,
		WindowStartUnixSeconds: windowStartUnixSeconds,
		WindowEndUnixSeconds:   windowEndUnixSeconds,
		LocationLat:            lat,
		LocationLng:            lng,
		LocationLabel:          locationLabel,
		Capacity:               capacity,
	})
	if err != nil {
		return Meetup{}, err
	}
	return meetupFromProto(resp), nil
}

func (c *grpcClient) ListOpenMeetups(ctx context.Context, userID, intent, cursor string, pageSize int32) ([]Meetup, string, error) {
	protoIntent, ok := intentToProto[intent]
	if !ok {
		return nil, "", fmt.Errorf("meetupclient: unrecognized intent %q", intent)
	}
	resp, err := c.client.ListOpenMeetups(ctx, &meetupv1.ListOpenMeetupsRequest{
		UserId:   userID,
		Intent:   protoIntent,
		Cursor:   cursor,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, "", err
	}
	meetups := make([]Meetup, 0, len(resp.GetMeetups()))
	for _, m := range resp.GetMeetups() {
		meetups = append(meetups, meetupFromProto(m))
	}
	return meetups, resp.GetNextCursor(), nil
}

func (c *grpcClient) GetMeetup(ctx context.Context, meetupID, userID string) (Meetup, error) {
	resp, err := c.client.GetMeetup(ctx, &meetupv1.GetMeetupRequest{MeetupId: meetupID, UserId: userID})
	if err != nil {
		return Meetup{}, err
	}
	return meetupFromProto(resp), nil
}

func (c *grpcClient) ListMyMeetups(ctx context.Context, userID string) ([]Meetup, []Meetup, error) {
	resp, err := c.client.ListMyMeetups(ctx, &meetupv1.ListMyMeetupsRequest{UserId: userID})
	if err != nil {
		return nil, nil, err
	}
	hosted := make([]Meetup, 0, len(resp.GetHosted()))
	for _, m := range resp.GetHosted() {
		hosted = append(hosted, meetupFromProto(m))
	}
	requested := make([]Meetup, 0, len(resp.GetRequested()))
	for _, m := range resp.GetRequested() {
		requested = append(requested, meetupFromProto(m))
	}
	return hosted, requested, nil
}

func (c *grpcClient) ListMeetupRequests(ctx context.Context, meetupID, hostUserID string) ([]MeetupRequest, error) {
	resp, err := c.client.ListMeetupRequests(ctx, &meetupv1.ListMeetupRequestsRequest{MeetupId: meetupID, HostUserId: hostUserID})
	if err != nil {
		return nil, err
	}
	requests := make([]MeetupRequest, 0, len(resp.GetRequests()))
	for _, r := range resp.GetRequests() {
		requests = append(requests, requestFromProto(r))
	}
	return requests, nil
}

func (c *grpcClient) RequestToJoin(ctx context.Context, meetupID, requesterID string, requesterTrustLevel int32) (MeetupRequest, error) {
	resp, err := c.client.RequestToJoin(ctx, &meetupv1.RequestToJoinRequest{
		MeetupId:            meetupID,
		RequesterId:         requesterID,
		RequesterTrustLevel: requesterTrustLevel,
	})
	if err != nil {
		return MeetupRequest{}, err
	}
	return requestFromProto(resp), nil
}

func (c *grpcClient) WithdrawRequest(ctx context.Context, requestID, requesterID string) error {
	_, err := c.client.WithdrawRequest(ctx, &meetupv1.WithdrawRequestRequest{RequestId: requestID, RequesterId: requesterID})
	return err
}

func (c *grpcClient) RespondToRequest(ctx context.Context, requestID, hostUserID string, accept bool) (MeetupRequest, error) {
	resp, err := c.client.RespondToRequest(ctx, &meetupv1.RespondToRequestRequest{
		RequestId:  requestID,
		HostUserId: hostUserID,
		Accept:     accept,
	})
	if err != nil {
		return MeetupRequest{}, err
	}
	return requestFromProto(resp), nil
}

func (c *grpcClient) RegisterDeviceToken(ctx context.Context, userID, fcmToken string) error {
	_, err := c.client.RegisterDeviceToken(ctx, &meetupv1.RegisterDeviceTokenRequest{UserId: userID, FcmToken: fcmToken})
	return err
}

func (c *grpcClient) GetSafetyState(ctx context.Context, meetupID string) (SafetyState, error) {
	resp, err := c.client.GetSafetyState(ctx, &meetupv1.GetSafetyStateRequest{MeetupId: meetupID})
	if err != nil {
		return SafetyState{}, err
	}
	return safetyStateFromProto(resp), nil
}

func (c *grpcClient) AcknowledgeSafetyChecklist(ctx context.Context, meetupID, userID string) (SafetyState, error) {
	resp, err := c.client.AcknowledgeSafetyChecklist(ctx, &meetupv1.AcknowledgeSafetyChecklistRequest{MeetupId: meetupID, UserId: userID})
	if err != nil {
		return SafetyState{}, err
	}
	return safetyStateFromProto(resp), nil
}

func (c *grpcClient) SetLiveLocationOptIn(ctx context.Context, meetupID, userID string, optIn bool) (SafetyState, error) {
	resp, err := c.client.SetLiveLocationOptIn(ctx, &meetupv1.SetLiveLocationOptInRequest{MeetupId: meetupID, UserId: userID, OptIn: optIn})
	if err != nil {
		return SafetyState{}, err
	}
	return safetyStateFromProto(resp), nil
}

func (c *grpcClient) CheckIn(ctx context.Context, meetupID, userID string) (SafetyState, error) {
	resp, err := c.client.CheckIn(ctx, &meetupv1.CheckInRequest{MeetupId: meetupID, UserId: userID})
	if err != nil {
		return SafetyState{}, err
	}
	return safetyStateFromProto(resp), nil
}

func (c *grpcClient) SubmitMeetupFeedback(
	ctx context.Context, meetupID, userID string, happened bool, feltSafe, profileAccurate, wouldMeetAgain *bool, notes *string,
) error {
	_, err := c.client.SubmitMeetupFeedback(ctx, &meetupv1.SubmitMeetupFeedbackRequest{
		MeetupId:        meetupID,
		UserId:          userID,
		Happened:        happened,
		FeltSafe:        feltSafe,
		ProfileAccurate: profileAccurate,
		WouldMeetAgain:  wouldMeetAgain,
		Notes:           notes,
	})
	return err
}

func (c *grpcClient) ListRatableParticipants(ctx context.Context, meetupID, viewerID string) ([]RatableParticipant, error) {
	resp, err := c.client.ListRatableParticipants(ctx, &meetupv1.ListRatableParticipantsRequest{MeetupId: meetupID, ViewerId: viewerID})
	if err != nil {
		return nil, err
	}
	participants := make([]RatableParticipant, 0, len(resp.GetParticipants()))
	for _, p := range resp.GetParticipants() {
		participants = append(participants, RatableParticipant{
			UserID:          p.GetUserId(),
			FullName:        p.GetFullName(),
			ProfilePhotoURL: p.GetProfilePhotoUrl(),
			TrustLevel:      p.GetTrustLevel(),
			AlreadyRated:    p.GetAlreadyRated(),
		})
	}
	return participants, nil
}

func (c *grpcClient) SubmitRating(ctx context.Context, meetupID, raterUserID, ratedUserID string, score int32) error {
	_, err := c.client.SubmitRating(ctx, &meetupv1.SubmitRatingRequest{
		MeetupId:    meetupID,
		RaterUserId: raterUserID,
		RatedUserId: ratedUserID,
		Score:       score,
	})
	return err
}

func (c *grpcClient) CloseMeetup(ctx context.Context, meetupID, hostUserID string) (Meetup, error) {
	resp, err := c.client.CloseMeetup(ctx, &meetupv1.CloseMeetupRequest{MeetupId: meetupID, HostUserId: hostUserID})
	if err != nil {
		return Meetup{}, err
	}
	return meetupFromProto(resp.GetMeetup()), nil
}

func (c *grpcClient) CancelMeetup(ctx context.Context, meetupID, hostUserID string) error {
	_, err := c.client.CancelMeetup(ctx, &meetupv1.CancelMeetupRequest{MeetupId: meetupID, HostUserId: hostUserID})
	return err
}

var intentToProto = map[string]meetupv1.Intent{
	"coffee":     meetupv1.Intent_INTENT_COFFEE,
	"lunch":      meetupv1.Intent_INTENT_LUNCH,
	"networking": meetupv1.Intent_INTENT_NETWORKING,
	"mentorship": meetupv1.Intent_INTENT_MENTORSHIP,
	"ride_share": meetupv1.Intent_INTENT_RIDE_SHARE,
	"dating":     meetupv1.Intent_INTENT_DATING,
}

var intentFromProto = map[meetupv1.Intent]string{
	meetupv1.Intent_INTENT_COFFEE:     "coffee",
	meetupv1.Intent_INTENT_LUNCH:      "lunch",
	meetupv1.Intent_INTENT_NETWORKING: "networking",
	meetupv1.Intent_INTENT_MENTORSHIP: "mentorship",
	meetupv1.Intent_INTENT_RIDE_SHARE: "ride_share",
	meetupv1.Intent_INTENT_DATING:     "dating",
}

var statusFromProto = map[meetupv1.MeetupStatus]string{
	meetupv1.MeetupStatus_MEETUP_STATUS_OPEN:      "open",
	meetupv1.MeetupStatus_MEETUP_STATUS_FULL:      "full",
	meetupv1.MeetupStatus_MEETUP_STATUS_CANCELLED: "cancelled",
	meetupv1.MeetupStatus_MEETUP_STATUS_COMPLETED: "completed",
}

var requestStatusFromProto = map[meetupv1.MeetupRequestStatus]string{
	meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_PENDING:   "pending",
	meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_ACCEPTED:  "accepted",
	meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_REJECTED:  "rejected",
	meetupv1.MeetupRequestStatus_MEETUP_REQUEST_STATUS_WITHDRAWN: "withdrawn",
}

func meetupFromProto(m *meetupv1.MeetupResponse) Meetup {
	out := Meetup{
		ID:                     m.GetId(),
		HostUserID:             m.GetHostUserId(),
		HostFullName:           m.GetHostFullName(),
		HostProfilePhotoURL:    m.GetHostProfilePhotoUrl(),
		HostTrustLevel:         m.GetHostTrustLevel(),
		HostRatingAverage:      m.GetHostRatingAverage(),
		HostRatingCount:        m.GetHostRatingCount(),
		Intent:                 intentFromProto[m.GetIntent()],
		WindowStartUnixSeconds: m.GetWindowStartUnixSeconds(),
		WindowEndUnixSeconds:   m.GetWindowEndUnixSeconds(),
		LocationLat:            m.GetLocationLat(),
		LocationLng:            m.GetLocationLng(),
		LocationLabel:          m.GetLocationLabel(),
		Capacity:               m.GetCapacity(),
		AcceptedCount:          m.GetAcceptedCount(),
		Status:                 statusFromProto[m.GetStatus()],
		CreatedAtUnixSeconds:   m.GetCreatedAtUnixSeconds(),
		CancelledAtUnixSeconds: m.CancelledAtUnixSeconds,
		ClosedAtUnixSeconds:    m.ClosedAtUnixSeconds,
		IsHostedByMe:           m.GetIsHostedByMe(),
	}
	if m.MyRequestStatus != nil {
		status := requestStatusFromProto[m.GetMyRequestStatus()]
		out.MyRequestStatus = &status
		out.MyRequestAutoRejected = m.GetMyRequestAutoRejected()
	}
	return out
}

func requestFromProto(r *meetupv1.MeetupRequestResponse) MeetupRequest {
	return MeetupRequest{
		ID:                       r.GetId(),
		MeetupID:                 r.GetMeetupId(),
		RequesterID:              r.GetRequesterId(),
		RequesterFullName:        r.GetRequesterFullName(),
		RequesterProfilePhotoURL: r.GetRequesterProfilePhotoUrl(),
		RequesterTrustLevel:      r.GetRequesterTrustLevel(),
		RequesterRatingAverage:   r.GetRequesterRatingAverage(),
		RequesterRatingCount:     r.GetRequesterRatingCount(),
		Status:                   requestStatusFromProto[r.GetStatus()],
		AutoRejected:             r.GetAutoRejected(),
		CreatedAtUnixSeconds:     r.GetCreatedAtUnixSeconds(),
		ResolvedAtUnixSeconds:    r.ResolvedAtUnixSeconds,
	}
}

func safetyStateFromProto(s *meetupv1.SafetyStateResponse) SafetyState {
	return SafetyState{
		MeetupID:                  s.GetMeetupId(),
		ChecklistAckAtUnixSeconds: s.ChecklistAckAtUnixSeconds,
		LiveLocationOptIn:         s.GetLiveLocationOptIn(),
		CheckedInAtUnixSeconds:    s.CheckedInAtUnixSeconds,
	}
}
