package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/professional-connections/backend/services/gateway/internal/meetupclient"
	"github.com/professional-connections/backend/services/gateway/internal/middleware"
)

// meetupResponse is the REST shape for a meetup — Intent/Status/
// MyRequestStatus are plain lowercase strings (meetupclient already
// converted them from proto enums), matching the frontend's IntentType
// wire format.
type meetupResponse struct {
	ID                     string  `json:"id"`
	HostUserID             string  `json:"host_user_id"`
	HostFullName           string  `json:"host_full_name"`
	HostProfilePhotoURL    string  `json:"host_profile_photo_url"`
	HostTrustLevel         int32   `json:"host_trust_level"`
	HostRatingAverage      float64 `json:"host_rating_average"`
	HostRatingCount        int32   `json:"host_rating_count"`
	Intent                 string  `json:"intent"`
	WindowStartUnixSeconds int64   `json:"window_start_unix_seconds"`
	WindowEndUnixSeconds   int64   `json:"window_end_unix_seconds"`
	LocationLat            float64 `json:"location_lat"`
	LocationLng            float64 `json:"location_lng"`
	LocationLabel          string  `json:"location_label"`
	Capacity               int32   `json:"capacity"`
	AcceptedCount          int32   `json:"accepted_count"`
	Status                 string  `json:"status"`
	CreatedAtUnixSeconds   int64   `json:"created_at_unix_seconds"`
	CancelledAtUnixSeconds *int64  `json:"cancelled_at_unix_seconds,omitempty"`
	ClosedAtUnixSeconds    *int64  `json:"closed_at_unix_seconds,omitempty"`
	IsHostedByMe           bool    `json:"is_hosted_by_me"`
	MyRequestStatus        *string `json:"my_request_status,omitempty"`
	MyRequestAutoRejected  bool    `json:"my_request_auto_rejected,omitempty"`
}

func meetupFromClient(m meetupclient.Meetup) meetupResponse {
	return meetupResponse{
		ID:                     m.ID,
		HostUserID:             m.HostUserID,
		HostFullName:           m.HostFullName,
		HostProfilePhotoURL:    m.HostProfilePhotoURL,
		HostTrustLevel:         m.HostTrustLevel,
		HostRatingAverage:      m.HostRatingAverage,
		HostRatingCount:        m.HostRatingCount,
		Intent:                 m.Intent,
		WindowStartUnixSeconds: m.WindowStartUnixSeconds,
		WindowEndUnixSeconds:   m.WindowEndUnixSeconds,
		LocationLat:            m.LocationLat,
		LocationLng:            m.LocationLng,
		LocationLabel:          m.LocationLabel,
		Capacity:               m.Capacity,
		AcceptedCount:          m.AcceptedCount,
		Status:                 m.Status,
		CreatedAtUnixSeconds:   m.CreatedAtUnixSeconds,
		CancelledAtUnixSeconds: m.CancelledAtUnixSeconds,
		ClosedAtUnixSeconds:    m.ClosedAtUnixSeconds,
		IsHostedByMe:           m.IsHostedByMe,
		MyRequestStatus:        m.MyRequestStatus,
		MyRequestAutoRejected:  m.MyRequestAutoRejected,
	}
}

func meetupsFromClient(meetups []meetupclient.Meetup) []meetupResponse {
	out := make([]meetupResponse, 0, len(meetups))
	for _, m := range meetups {
		out = append(out, meetupFromClient(m))
	}
	return out
}

type meetupRequestResponse struct {
	ID                       string  `json:"id"`
	MeetupID                 string  `json:"meetup_id"`
	RequesterID              string  `json:"requester_id"`
	RequesterFullName        string  `json:"requester_full_name"`
	RequesterProfilePhotoURL string  `json:"requester_profile_photo_url"`
	RequesterTrustLevel      int32   `json:"requester_trust_level"`
	RequesterRatingAverage   float64 `json:"requester_rating_average"`
	RequesterRatingCount     int32   `json:"requester_rating_count"`
	Status                   string  `json:"status"`
	AutoRejected             bool    `json:"auto_rejected"`
	CreatedAtUnixSeconds     int64   `json:"created_at_unix_seconds"`
	ResolvedAtUnixSeconds    *int64  `json:"resolved_at_unix_seconds,omitempty"`
}

func requestFromClient(r meetupclient.MeetupRequest) meetupRequestResponse {
	return meetupRequestResponse{
		ID:                       r.ID,
		MeetupID:                 r.MeetupID,
		RequesterID:              r.RequesterID,
		RequesterFullName:        r.RequesterFullName,
		RequesterProfilePhotoURL: r.RequesterProfilePhotoURL,
		RequesterTrustLevel:      r.RequesterTrustLevel,
		RequesterRatingAverage:   r.RequesterRatingAverage,
		RequesterRatingCount:     r.RequesterRatingCount,
		Status:                   r.Status,
		AutoRejected:             r.AutoRejected,
		CreatedAtUnixSeconds:     r.CreatedAtUnixSeconds,
		ResolvedAtUnixSeconds:    r.ResolvedAtUnixSeconds,
	}
}

type safetyStateResponse struct {
	MeetupID                  string `json:"meetup_id"`
	ChecklistAckAtUnixSeconds *int64 `json:"checklist_ack_at_unix_seconds,omitempty"`
	LiveLocationOptIn         bool   `json:"live_location_opt_in"`
	CheckedInAtUnixSeconds    *int64 `json:"checked_in_at_unix_seconds,omitempty"`
}

func safetyStateFromClient(s meetupclient.SafetyState) safetyStateResponse {
	return safetyStateResponse{
		MeetupID:                  s.MeetupID,
		ChecklistAckAtUnixSeconds: s.ChecklistAckAtUnixSeconds,
		LiveLocationOptIn:         s.LiveLocationOptIn,
		CheckedInAtUnixSeconds:    s.CheckedInAtUnixSeconds,
	}
}

type createMeetupRequest struct {
	Intent                 string  `json:"intent"`
	WindowStartUnixSeconds int64   `json:"window_start_unix_seconds"`
	WindowEndUnixSeconds   int64   `json:"window_end_unix_seconds"`
	LocationLat            float64 `json:"location_lat"`
	LocationLng            float64 `json:"location_lng"`
	LocationLabel          string  `json:"location_label"`
	Capacity               int32   `json:"capacity"`
}

func (h *Handler) createMeetup(w http.ResponseWriter, r *http.Request) {
	var req createMeetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	m, err := h.meetup.CreateMeetup(ctx, middleware.UserIDFromContext(ctx), int32(middleware.TrustLevelFromContext(ctx)),
		req.Intent, req.WindowStartUnixSeconds, req.WindowEndUnixSeconds, req.LocationLat, req.LocationLng, req.LocationLabel, req.Capacity)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meetupFromClient(m))
}

// listOpenMeetupsResponse wraps the page + next_cursor, matching the
// frontend's existing PagedResult shape (frontend/meetup-scheduling-
// PLAN.md Step 4).
type listOpenMeetupsResponse struct {
	Meetups    []meetupResponse `json:"meetups"`
	NextCursor string           `json:"next_cursor"`
}

func (h *Handler) listOpenMeetups(w http.ResponseWriter, r *http.Request) {
	intent := r.URL.Query().Get("intent")
	if intent == "" {
		writeError(w, http.StatusBadRequest, "intent query parameter is required")
		return
	}
	cursor := r.URL.Query().Get("cursor")
	pageSize := int32(0)
	if raw := r.URL.Query().Get("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "page_size must be an integer")
			return
		}
		pageSize = int32(parsed)
	}

	ctx := r.Context()
	meetups, nextCursor, err := h.meetup.ListOpenMeetups(ctx, middleware.UserIDFromContext(ctx), intent, cursor, pageSize)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listOpenMeetupsResponse{Meetups: meetupsFromClient(meetups), NextCursor: nextCursor})
}

func (h *Handler) getMeetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m, err := h.meetup.GetMeetup(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meetupFromClient(m))
}

type listMyMeetupsResponse struct {
	Hosted    []meetupResponse `json:"hosted"`
	Requested []meetupResponse `json:"requested"`
}

func (h *Handler) listMyMeetups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hosted, requested, err := h.meetup.ListMyMeetups(ctx, middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listMyMeetupsResponse{Hosted: meetupsFromClient(hosted), Requested: meetupsFromClient(requested)})
}

type listMeetupRequestsResponse struct {
	Requests []meetupRequestResponse `json:"requests"`
}

func (h *Handler) listMeetupRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requests, err := h.meetup.ListMeetupRequests(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]meetupRequestResponse, 0, len(requests))
	for _, req := range requests {
		out = append(out, requestFromClient(req))
	}
	writeJSON(w, http.StatusOK, listMeetupRequestsResponse{Requests: out})
}

func (h *Handler) requestToJoin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := h.meetup.RequestToJoin(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx), int32(middleware.TrustLevelFromContext(ctx)))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requestFromClient(req))
}

type successResponse struct {
	Success bool `json:"success"`
}

func (h *Handler) withdrawRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.meetup.WithdrawRequest(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx)); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, successResponse{Success: true})
}

type respondToRequestRequest struct {
	Accept bool `json:"accept"`
}

func (h *Handler) respondToRequest(w http.ResponseWriter, r *http.Request) {
	var req respondToRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	updated, err := h.meetup.RespondToRequest(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx), req.Accept)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requestFromClient(updated))
}

type registerDeviceTokenRequest struct {
	FcmToken string `json:"fcm_token"`
}

func (h *Handler) registerDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req registerDeviceTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.meetup.RegisterDeviceToken(ctx, middleware.UserIDFromContext(ctx), req.FcmToken); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, successResponse{Success: true})
}

func (h *Handler) getSafetyState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state, err := h.meetup.GetSafetyState(ctx, r.PathValue("id"))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, safetyStateFromClient(state))
}

func (h *Handler) acknowledgeSafetyChecklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state, err := h.meetup.AcknowledgeSafetyChecklist(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, safetyStateFromClient(state))
}

type setLiveLocationOptInRequest struct {
	OptIn bool `json:"opt_in"`
}

func (h *Handler) setLiveLocationOptIn(w http.ResponseWriter, r *http.Request) {
	var req setLiveLocationOptInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	state, err := h.meetup.SetLiveLocationOptIn(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx), req.OptIn)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, safetyStateFromClient(state))
}

func (h *Handler) checkIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state, err := h.meetup.CheckIn(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, safetyStateFromClient(state))
}

type submitMeetupFeedbackRequest struct {
	Happened        bool    `json:"happened"`
	FeltSafe        *bool   `json:"felt_safe"`
	ProfileAccurate *bool   `json:"profile_accurate"`
	WouldMeetAgain  *bool   `json:"would_meet_again"`
	Notes           *string `json:"notes"`
}

func (h *Handler) submitMeetupFeedback(w http.ResponseWriter, r *http.Request) {
	var req submitMeetupFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.meetup.SubmitMeetupFeedback(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx),
		req.Happened, req.FeltSafe, req.ProfileAccurate, req.WouldMeetAgain, req.Notes); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, successResponse{Success: true})
}

// ratableParticipantResponse mirrors meetupclient.RatableParticipant
// (ADR-015) — the host + accepted requesters of a meetup, minus the viewer,
// each flagged with whether the viewer already rated them.
type ratableParticipantResponse struct {
	UserID          string `json:"user_id"`
	FullName        string `json:"full_name"`
	ProfilePhotoURL string `json:"profile_photo_url"`
	TrustLevel      int32  `json:"trust_level"`
	AlreadyRated    bool   `json:"already_rated"`
}

type listRatableParticipantsResponse struct {
	Participants []ratableParticipantResponse `json:"participants"`
}

func (h *Handler) listRatableParticipants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	participants, err := h.meetup.ListRatableParticipants(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	out := make([]ratableParticipantResponse, 0, len(participants))
	for _, p := range participants {
		out = append(out, ratableParticipantResponse{
			UserID:          p.UserID,
			FullName:        p.FullName,
			ProfilePhotoURL: p.ProfilePhotoURL,
			TrustLevel:      p.TrustLevel,
			AlreadyRated:    p.AlreadyRated,
		})
	}
	writeJSON(w, http.StatusOK, listRatableParticipantsResponse{Participants: out})
}

type submitRatingRequest struct {
	RatedUserID string `json:"rated_user_id"`
	Score       int32  `json:"score"`
}

func (h *Handler) submitRating(w http.ResponseWriter, r *http.Request) {
	var req submitRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	if err := h.meetup.SubmitRating(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx), req.RatedUserID, req.Score); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, successResponse{Success: true})
}

func (h *Handler) closeMeetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m, err := h.meetup.CloseMeetup(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meetupFromClient(m))
}

func (h *Handler) cancelMeetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.meetup.CancelMeetup(ctx, r.PathValue("id"), middleware.UserIDFromContext(ctx)); err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, successResponse{Success: true})
}
