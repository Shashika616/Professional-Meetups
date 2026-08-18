package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/professional-connections/backend/services/gateway/internal/middleware"
)

type startVerificationResponse struct {
	ResendAfterSeconds int32 `json:"resend_after_seconds"`
}

type phoneStartRequest struct {
	PhoneNumber string `json:"phone_number"`
}

func (h *Handler) startPhoneVerification(w http.ResponseWriter, r *http.Request) {
	var req phoneStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resendAfter, err := h.auth.StartPhoneVerification(r.Context(), middleware.UserIDFromContext(r.Context()), req.PhoneNumber)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, startVerificationResponse{ResendAfterSeconds: resendAfter})
}

type phoneVerifyRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
}

func (h *Handler) verifyPhoneCode(w http.ResponseWriter, r *http.Request) {
	var req phoneVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.VerifyPhoneCode(r.Context(), middleware.UserIDFromContext(r.Context()), req.PhoneNumber, req.Code)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type emailStartRequest struct {
	Email string `json:"email"`
}

func (h *Handler) startPersonalEmailVerification(w http.ResponseWriter, r *http.Request) {
	var req emailStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resendAfter, err := h.auth.StartPersonalEmailVerification(r.Context(), middleware.UserIDFromContext(r.Context()), req.Email)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, startVerificationResponse{ResendAfterSeconds: resendAfter})
}

type emailVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handler) verifyPersonalEmailCode(w http.ResponseWriter, r *http.Request) {
	var req emailVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.VerifyPersonalEmailCode(r.Context(), middleware.UserIDFromContext(r.Context()), req.Email, req.Code)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

type personalDetailsRequest struct {
	LegalName string `json:"legal_name"`
	Address   string `json:"address"`
}

func (h *Handler) submitPersonalDetails(w http.ResponseWriter, r *http.Request) {
	var req personalDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.SubmitPersonalDetails(r.Context(), middleware.UserIDFromContext(r.Context()), req.LegalName, req.Address)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

func (h *Handler) startCorporateEmailVerification(w http.ResponseWriter, r *http.Request) {
	var req emailStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resendAfter, err := h.auth.StartCorporateEmailVerification(r.Context(), middleware.UserIDFromContext(r.Context()), req.Email)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, startVerificationResponse{ResendAfterSeconds: resendAfter})
}

func (h *Handler) verifyCorporateEmailCode(w http.ResponseWriter, r *http.Request) {
	var req emailVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.VerifyCorporateEmailCode(r.Context(), middleware.UserIDFromContext(r.Context()), req.Email, req.Code)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionFromClient(session))
}

// profileResponse never carries a raw phone number or email address —
// booleans/derived fields only (Verification Model § 1).
type profileResponse struct {
	UserID                  string `json:"user_id"`
	FullName                string `json:"full_name"`
	ProfilePhotoURL         string `json:"profile_photo_url"`
	TrustLevel              int    `json:"trust_level"`
	PhoneVerified           bool   `json:"phone_verified"`
	PersonalEmailVerified   bool   `json:"personal_email_verified"`
	PersonalDetailsComplete bool   `json:"personal_details_complete"`
	CompanyDomain           string `json:"company_domain"`
	WorkEmailVerified       bool   `json:"work_email_verified"`
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := h.auth.GetProfile(r.Context(), middleware.UserIDFromContext(r.Context()))
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profileResponse{
		UserID:                  profile.UserID,
		FullName:                profile.FullName,
		ProfilePhotoURL:         profile.ProfilePhotoURL,
		TrustLevel:              profile.TrustLevel,
		PhoneVerified:           profile.PhoneVerified,
		PersonalEmailVerified:   profile.PersonalEmailVerified,
		PersonalDetailsComplete: profile.PersonalDetailsComplete,
		CompanyDomain:           profile.CompanyDomain,
		WorkEmailVerified:       profile.WorkEmailVerified,
	})
}
