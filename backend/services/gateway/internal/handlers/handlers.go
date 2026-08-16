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
	"github.com/professional-connections/backend/shared/apperror"
)

// Handler holds the gateway's REST endpoint implementations.
type Handler struct {
	auth authclient.Client
}

// New constructs a Handler.
func New(auth authclient.Client) *Handler {
	return &Handler{auth: auth}
}

// Register wires this Handler's routes onto mux, using Go 1.22+'s built-in
// method+path pattern matching — no third-party router needed at this API
// size (PLAN.md Step 5).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/linkedin/callback", h.linkedInCallback)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /v1/auth/logout", h.logout)
}

type linkedInCallbackRequest struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
	RedirectURI       string `json:"redirect_uri"`
}

type sessionResponse struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IsNewUser    bool   `json:"is_new_user"`
}

func (h *Handler) linkedInCallback(w http.ResponseWriter, r *http.Request) {
	var req linkedInCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.CompleteLinkedInOnboarding(r.Context(), req.AuthorizationCode, req.CodeVerifier, req.RedirectURI)
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
		UserID:       s.UserID,
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		ExpiresIn:    s.AccessTokenExpiresInSeconds,
		IsNewUser:    s.IsNewUser,
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
