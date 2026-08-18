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
	"github.com/professional-connections/backend/services/gateway/internal/middleware"
	"github.com/professional-connections/backend/shared/apperror"
	"github.com/professional-connections/backend/shared/jwt"
)

// Handler holds the gateway's REST endpoint implementations.
type Handler struct {
	auth        authclient.Client
	requireAuth func(http.Handler) http.Handler
}

// New constructs a Handler. verifier backs the auth middleware applied to
// the /v1/verification/* and /v1/users/me routes (Register) — every other
// route stays unauthenticated at the gateway layer, as before.
func New(auth authclient.Client, verifier *jwt.Verifier) *Handler {
	return &Handler{auth: auth, requireAuth: middleware.Auth(verifier)}
}

// Register wires this Handler's routes onto mux, using Go 1.22+'s built-in
// method+path pattern matching — no third-party router needed at this API
// size (PLAN.md Step 5). The /v1/verification/* and /v1/users/me routes are
// wrapped individually with h.requireAuth here — not applied globally in
// cmd/server/main.go's middleware chain — so the LinkedIn/refresh/logout
// routes stay unauthenticated at the gateway layer exactly as before
// (backend/PLAN.md's Level 2/3 addendum, Step F).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/linkedin/callback", h.linkedInCallback)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)

	mux.Handle("POST /v1/verification/phone/start", h.requireAuth(http.HandlerFunc(h.startPhoneVerification)))
	mux.Handle("POST /v1/verification/phone/verify", h.requireAuth(http.HandlerFunc(h.verifyPhoneCode)))
	mux.Handle("POST /v1/verification/personal-email/start", h.requireAuth(http.HandlerFunc(h.startPersonalEmailVerification)))
	mux.Handle("POST /v1/verification/personal-email/verify", h.requireAuth(http.HandlerFunc(h.verifyPersonalEmailCode)))
	mux.Handle("POST /v1/verification/personal-details", h.requireAuth(http.HandlerFunc(h.submitPersonalDetails)))
	mux.Handle("POST /v1/verification/corporate-email/start", h.requireAuth(http.HandlerFunc(h.startCorporateEmailVerification)))
	mux.Handle("POST /v1/verification/corporate-email/verify", h.requireAuth(http.HandlerFunc(h.verifyCorporateEmailCode)))
	mux.Handle("GET /v1/users/me", h.requireAuth(http.HandlerFunc(h.getProfile)))
}

type linkedInCallbackRequest struct {
	AuthorizationCode string `json:"authorization_code"`
	RedirectURI       string `json:"redirect_uri"`
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

func (h *Handler) linkedInCallback(w http.ResponseWriter, r *http.Request) {
	var req linkedInCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.CompleteLinkedInOnboarding(r.Context(), req.AuthorizationCode, req.RedirectURI)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionFromClient(session))
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
