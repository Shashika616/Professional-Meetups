// Package handlers implements the gateway's public REST API — the only
// place the public contract (JSON over HTTP) is translated to/from the
// internal gRPC contract (via internal/authclient). See PLAN.md Step 5 for
// the full REST contract.
package handlers

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/status"

	"github.com/professional-connections/backend/services/gateway/internal/authclient"
	"github.com/professional-connections/backend/services/gateway/internal/meetupclient"
	"github.com/professional-connections/backend/services/gateway/internal/middleware"
	"github.com/professional-connections/backend/shared/apperror"
	"github.com/professional-connections/backend/shared/jwt"
)

// Handler holds the gateway's REST endpoint implementations.
type Handler struct {
	auth        authclient.Client
	meetup      meetupclient.Client
	requireAuth func(http.Handler) http.Handler
}

// New constructs a Handler. verifier backs the auth middleware applied to
// the /v1/verification/*, /v1/users/me, and /v1/meetups/* routes
// (Register) — every other route stays unauthenticated at the gateway
// layer, as before.
func New(auth authclient.Client, meetup meetupclient.Client, verifier *jwt.Verifier) *Handler {
	return &Handler{auth: auth, meetup: meetup, requireAuth: middleware.Auth(verifier)}
}

// Register wires this Handler's routes onto mux, using Go 1.22+'s built-in
// method+path pattern matching — no third-party router needed at this API
// size (PLAN.md Step 5). The /v1/verification/* and /v1/users/me routes are
// wrapped individually with h.requireAuth here — not applied globally in
// cmd/server/main.go's middleware chain — so the LinkedIn/refresh/logout
// routes stay unauthenticated at the gateway layer exactly as before
// (backend/PLAN.md's Level 2/3 addendum, Step F).
func (h *Handler) Register(mux *http.ServeMux) {
	// Unauthenticated (ADR-014) — four parallel, co-equal account-creation
	// paths. federated/signup (Apple/Google) and linkedin/callback
	// (unchanged from ADR-011) are both resolve-or-create; email signup is
	// a two-step OTP-then-create flow (StartEmailSignup/CompleteEmailSignup
	// mirror the existing verification start/verify route-pair convention
	// used below), and email/login is the one path needing its own sign-in
	// form since a password can't collapse resolve-or-create into one tap.
	mux.HandleFunc("POST /v1/auth/federated/signup", h.federatedSignup)
	mux.HandleFunc("POST /v1/auth/linkedin/callback", h.linkedInCallback)
	mux.HandleFunc("POST /v1/auth/email/signup/start", h.startEmailSignup)
	mux.HandleFunc("POST /v1/auth/email/signup", h.completeEmailSignup)
	mux.HandleFunc("POST /v1/auth/email/login", h.emailLogin)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)

	// Authenticated — Profile-initiated linking only (ADR-014's "Connect
	// LinkedIn" flow, or a future "add Apple/Google as backup sign-in").
	mux.Handle("POST /v1/auth/identities/link", h.requireAuth(http.HandlerFunc(h.linkIdentity)))

	mux.Handle("POST /v1/verification/phone/start", h.requireAuth(http.HandlerFunc(h.startPhoneVerification)))
	mux.Handle("POST /v1/verification/phone/verify", h.requireAuth(http.HandlerFunc(h.verifyPhoneCode)))
	mux.Handle("POST /v1/verification/personal-email/start", h.requireAuth(http.HandlerFunc(h.startPersonalEmailVerification)))
	mux.Handle("POST /v1/verification/personal-email/verify", h.requireAuth(http.HandlerFunc(h.verifyPersonalEmailCode)))
	mux.Handle("POST /v1/verification/personal-details", h.requireAuth(http.HandlerFunc(h.submitPersonalDetails)))
	mux.Handle("POST /v1/verification/corporate-email/start", h.requireAuth(http.HandlerFunc(h.startCorporateEmailVerification)))
	mux.Handle("POST /v1/verification/corporate-email/verify", h.requireAuth(http.HandlerFunc(h.verifyCorporateEmailCode)))
	mux.Handle("GET /v1/users/me", h.requireAuth(http.HandlerFunc(h.getProfile)))

	// Meetup scheduling & join requests (ADR-013, backend/meetup-
	// scheduling-PLAN.md Step D) — all authenticated, same requireAuth as
	// above. CreateMeetup/RequestToJoin get the same IP+path rate limit as
	// every other route registered on this mux (main.go wraps the whole
	// mux in middleware.RateLimit) — a spam-created-meetups or
	// spam-join-requests vector is the same shape of abuse as spam-OTP-
	// sends, and the existing limiter is already keyed per route path, so
	// no separate limiter is needed here.
	mux.Handle("POST /v1/meetups", h.requireAuth(http.HandlerFunc(h.createMeetup)))
	mux.Handle("GET /v1/meetups", h.requireAuth(http.HandlerFunc(h.listOpenMeetups)))
	mux.Handle("GET /v1/meetups/mine", h.requireAuth(http.HandlerFunc(h.listMyMeetups)))
	mux.Handle("GET /v1/meetups/{id}", h.requireAuth(http.HandlerFunc(h.getMeetup)))
	mux.Handle("POST /v1/meetups/{id}/close", h.requireAuth(http.HandlerFunc(h.closeMeetup)))
	mux.Handle("POST /v1/meetups/{id}/cancel", h.requireAuth(http.HandlerFunc(h.cancelMeetup)))
	mux.Handle("GET /v1/meetups/{id}/requests", h.requireAuth(http.HandlerFunc(h.listMeetupRequests)))
	mux.Handle("POST /v1/meetups/{id}/requests", h.requireAuth(http.HandlerFunc(h.requestToJoin)))
	mux.Handle("POST /v1/meetups/requests/{id}/withdraw", h.requireAuth(http.HandlerFunc(h.withdrawRequest)))
	mux.Handle("POST /v1/meetups/requests/{id}/respond", h.requireAuth(http.HandlerFunc(h.respondToRequest)))
	mux.Handle("POST /v1/meetups/device-token", h.requireAuth(http.HandlerFunc(h.registerDeviceToken)))
	mux.Handle("GET /v1/meetups/{id}/safety", h.requireAuth(http.HandlerFunc(h.getSafetyState)))
	mux.Handle("POST /v1/meetups/{id}/safety/checklist", h.requireAuth(http.HandlerFunc(h.acknowledgeSafetyChecklist)))
	mux.Handle("POST /v1/meetups/{id}/safety/live-location", h.requireAuth(http.HandlerFunc(h.setLiveLocationOptIn)))
	mux.Handle("POST /v1/meetups/{id}/safety/check-in", h.requireAuth(http.HandlerFunc(h.checkIn)))
	mux.Handle("POST /v1/meetups/{id}/feedback", h.requireAuth(http.HandlerFunc(h.submitMeetupFeedback)))
	mux.Handle("GET /v1/meetups/{id}/ratings/ratable", h.requireAuth(http.HandlerFunc(h.listRatableParticipants)))
	mux.Handle("POST /v1/meetups/{id}/ratings", h.requireAuth(http.HandlerFunc(h.submitRating)))
}

type federatedSignupRequest struct {
	Provider           string `json:"provider"` // "apple" | "google"
	IDToken            string `json:"id_token"`
	AgeConfirmedOver18 bool   `json:"age_confirmed_over_18"`
}

// federatedSignup creates a Level 0 account via Sign in with Apple/Google,
// or logs in if this (provider, subject) already has one (ADR-014).
// Unauthenticated — see Register's comment.
func (h *Handler) federatedSignup(w http.ResponseWriter, r *http.Request) {
	var req federatedSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.CompleteFederatedSignup(r.Context(), req.Provider, req.IDToken, req.AgeConfirmedOver18)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type linkedInCallbackRequest struct {
	AuthorizationCode  string `json:"authorization_code"`
	RedirectURI        string `json:"redirect_uri"`
	AgeConfirmedOver18 bool   `json:"age_confirmed_over_18"`
}

// linkedInCallback creates a Level 1 account directly via LinkedIn, or logs
// in if this linkedin_sub already has one — unchanged in behavior from
// ADR-011, unauthenticated (Register).
func (h *Handler) linkedInCallback(w http.ResponseWriter, r *http.Request) {
	var req linkedInCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.CompleteLinkedInOnboarding(r.Context(), req.AuthorizationCode, req.RedirectURI, req.AgeConfirmedOver18)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type linkIdentityRequest struct {
	Provider          string `json:"provider"` // "apple" | "google" | "linkedin"
	IDToken           string `json:"id_token"`
	AuthorizationCode string `json:"authorization_code"`
	RedirectURI       string `json:"redirect_uri"`
}

// linkIdentity links an identity to the caller's already-authenticated
// account (ADR-014's Profile "Connect LinkedIn" flow) — authenticated, see
// Register.
func (h *Handler) linkIdentity(w http.ResponseWriter, r *http.Request) {
	var req linkIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.LinkIdentity(
		r.Context(), middleware.UserIDFromContext(r.Context()), req.Provider, req.IDToken, req.AuthorizationCode, req.RedirectURI,
	)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type startEmailSignupRequest struct {
	Email string `json:"email"`
}

type startEmailSignupResponse struct {
	ResendAfterSeconds int32 `json:"resend_after_seconds"`
}

// startEmailSignup sends an OTP to email as the first step of the
// email+password signup flow (ADR-014 decision #2) — unauthenticated.
func (h *Handler) startEmailSignup(w http.ResponseWriter, r *http.Request) {
	var req startEmailSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resendAfterSeconds, err := h.auth.StartEmailSignup(r.Context(), req.Email)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, startEmailSignupResponse{ResendAfterSeconds: resendAfterSeconds})
}

type completeEmailSignupRequest struct {
	Email              string `json:"email"`
	Code               string `json:"code"`
	Password           string `json:"password"`
	AgeConfirmedOver18 bool   `json:"age_confirmed_over_18"`
}

// completeEmailSignup verifies the OTP sent by startEmailSignup and
// creates (or, per SignUpOrRecoverWithEmail, recovers) an account —
// unauthenticated.
func (h *Handler) completeEmailSignup(w http.ResponseWriter, r *http.Request) {
	var req completeEmailSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.CompleteEmailSignup(r.Context(), req.Email, req.Code, req.Password, req.AgeConfirmedOver18)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type emailLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// emailLogin signs in with email+password — unauthenticated.
func (h *Handler) emailLogin(w http.ResponseWriter, r *http.Request) {
	var req emailLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.LoginWithPassword(r.Context(), req.Email, req.Password)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type sessionResponse struct {
	UserID          string `json:"user_id"`
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int64  `json:"expires_in"`
	IsNewUser       bool   `json:"is_new_user"`
	FullName        string `json:"full_name"`
	ProfilePhotoURL string `json:"profile_photo_url"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.RefreshSession(r.Context(), req.RefreshToken)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutResponse struct {
	Success bool `json:"success"`
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// RevokeSession is idempotent all the way down to the repository layer
	// — an unknown or already-revoked token isn't an error, so this only
	// returns non-200 for a genuine backend failure.
	if err := h.auth.RevokeSession(r.Context(), req.RefreshToken); err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, logoutResponse{Success: true})
}

func sessionFromClient(s authclient.Session) sessionResponse {
	return sessionResponse{
		UserID:          s.UserID,
		AccessToken:     s.AccessToken,
		RefreshToken:    s.RefreshToken,
		ExpiresIn:       s.AccessTokenExpiresInSeconds,
		IsNewUser:       s.IsNewUser,
		FullName:        s.FullName,
		ProfilePhotoURL: s.ProfilePhotoURL,
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

// writeGRPCError maps a gRPC status error from authclient to the
// corresponding HTTP status via shared/apperror's single mapping table —
// the one place this translation happens, not a switch statement per
// handler.
func writeGRPCError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	writeError(w, apperror.HTTPStatusFromGRPC(st.Code()), st.Message())
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
