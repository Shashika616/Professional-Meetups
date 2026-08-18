package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/professional-connections/backend/services/auth/internal/email"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

// StartPhoneVerification generates and sends a phone OTP.
func (s *Service) StartPhoneVerification(ctx context.Context, req *authv1.StartVerificationRequest) (*authv1.StartVerificationResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for StartPhoneVerification: %w", apperror.ErrInvalidInput))
	}
	return s.startVerificationRPC(ctx, req.GetUserId(), repository.VerificationPurposePhone, req.GetTarget())
}

// VerifyPhoneCode verifies a phone OTP and, on success, persists
// phone_number and returns a fresh session reflecting the new trust level.
func (s *Service) VerifyPhoneCode(ctx context.Context, req *authv1.VerifyCodeRequest) (*authv1.SessionResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_PHONE {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for VerifyPhoneCode: %w", apperror.ErrInvalidInput))
	}
	if err := s.verifyAndConsumeCode(ctx, req.GetUserId(), repository.VerificationPurposePhone, req.GetTarget(), req.GetCode()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	hypothetical := user
	hypothetical.PhoneNumber = req.GetTarget()

	persisted, err := s.users.UpdatePhoneNumber(ctx, req.GetUserId(), req.GetTarget(), computeTrustLevel(hypothetical))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return s.sessionForVerifiedUser(ctx, persisted)
}

// StartPersonalEmailVerification generates and sends a personal-email OTP.
func (s *Service) StartPersonalEmailVerification(ctx context.Context, req *authv1.StartVerificationRequest) (*authv1.StartVerificationResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for StartPersonalEmailVerification: %w", apperror.ErrInvalidInput))
	}
	return s.startVerificationRPC(ctx, req.GetUserId(), repository.VerificationPurposePersonalEmail, req.GetTarget())
}

// VerifyPersonalEmailCode verifies a personal-email OTP and, on success,
// persists personal_email and returns a fresh session.
func (s *Service) VerifyPersonalEmailCode(ctx context.Context, req *authv1.VerifyCodeRequest) (*authv1.SessionResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_PERSONAL_EMAIL {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for VerifyPersonalEmailCode: %w", apperror.ErrInvalidInput))
	}
	if err := s.verifyAndConsumeCode(ctx, req.GetUserId(), repository.VerificationPurposePersonalEmail, req.GetTarget(), req.GetCode()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	hypothetical := user
	hypothetical.PersonalEmail = req.GetTarget()

	persisted, err := s.users.UpdatePersonalEmail(ctx, req.GetUserId(), req.GetTarget(), computeTrustLevel(hypothetical))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return s.sessionForVerifiedUser(ctx, persisted)
}

// SubmitPersonalDetails is the one Level 2 step with no OTP — legal
// name/address are self-reported (Verification Model § 4).
func (s *Service) SubmitPersonalDetails(ctx context.Context, req *authv1.SubmitPersonalDetailsRequest) (*authv1.SessionResponse, error) {
	if req.GetLegalName() == "" || req.GetAddress() == "" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("legal name and address are both required: %w", apperror.ErrInvalidInput))
	}

	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	hypothetical := user
	hypothetical.LegalName = req.GetLegalName()
	hypothetical.Address = req.GetAddress()

	persisted, err := s.users.UpdatePersonalDetails(ctx, req.GetUserId(), req.GetLegalName(), req.GetAddress(), computeTrustLevel(hypothetical))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return s.sessionForVerifiedUser(ctx, persisted)
}

// StartCorporateEmailVerification rejects free/role-based addresses before
// a code is ever generated (Verification Model § 5's existing lists, MVP-
// scoped per ADR-012), then generates and sends a corporate-email OTP.
func (s *Service) StartCorporateEmailVerification(ctx context.Context, req *authv1.StartVerificationRequest) (*authv1.StartVerificationResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for StartCorporateEmailVerification: %w", apperror.ErrInvalidInput))
	}
	// Safe to be specific here (unlike the enumeration-sensitive cases
	// below) — this only reveals something about the domain the user
	// themselves just typed, not about any account's existence.
	if isRejectedCorporateEmail(req.GetTarget()) {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("please use your work email, not a personal address: %w", apperror.ErrInvalidInput))
	}
	return s.startVerificationRPC(ctx, req.GetUserId(), repository.VerificationPurposeCorporateEmail, req.GetTarget())
}

// VerifyCorporateEmailCode verifies a corporate-email OTP and, on success,
// extracts+persists company_domain (never the raw address, ADR-003),
// marks work_email_verified, and returns a fresh session.
func (s *Service) VerifyCorporateEmailCode(ctx context.Context, req *authv1.VerifyCodeRequest) (*authv1.SessionResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_CORPORATE_EMAIL {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for VerifyCorporateEmailCode: %w", apperror.ErrInvalidInput))
	}
	if err := s.verifyAndConsumeCode(ctx, req.GetUserId(), repository.VerificationPurposeCorporateEmail, req.GetTarget(), req.GetCode()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	domain := domainFromEmail(req.GetTarget())
	hypothetical := user
	hypothetical.CompanyDomain = domain
	hypothetical.WorkEmailVerified = true

	persisted, err := s.users.UpdateWorkEmailVerified(ctx, req.GetUserId(), domain, true, time.Now(), computeTrustLevel(hypothetical))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return s.sessionForVerifiedUser(ctx, persisted)
}

// GetProfile returns everything ProfilePage needs to render real
// verification state — booleans/derived fields only, never a raw phone
// number or email address (Verification Model § 1).
func (s *Service) GetProfile(ctx context.Context, req *authv1.GetProfileRequest) (*authv1.ProfileResponse, error) {
	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return &authv1.ProfileResponse{
		UserId:                  user.ID,
		FullName:                user.FullName,
		ProfilePhotoUrl:         user.ProfilePhotoURL,
		TrustLevel:              int32(user.TrustLevel),
		PhoneVerified:           user.PhoneNumber != "",
		PersonalEmailVerified:   user.PersonalEmail != "",
		PersonalDetailsComplete: user.LegalName != "" && user.Address != "",
		CompanyDomain:           user.CompanyDomain,
		WorkEmailVerified:       user.WorkEmailVerified,
	}, nil
}

// startVerificationRPC is the shared Start* implementation — one send
// mechanism, three purposes (backend/PLAN.md's addendum, Step C/D).
func (s *Service) startVerificationRPC(
	ctx context.Context, userID string, purpose repository.VerificationPurpose, target string,
) (*authv1.StartVerificationResponse, error) {
	if target == "" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("target is required: %w", apperror.ErrInvalidInput))
	}

	existing, err := s.verificationCodes.Get(ctx, userID, purpose)
	if err == nil {
		if age := time.Since(existing.CreatedAt); age < otpResendCooldown {
			return nil, apperror.ToGRPCStatus(fmt.Errorf("please wait before requesting another code: %w", apperror.ErrRateLimited))
		}
	} else if !errors.Is(err, apperror.ErrNotFound) {
		return nil, apperror.ToGRPCStatus(err)
	}

	code, err := generateOTP()
	if err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("%w: %w", apperror.ErrInternal, err))
	}

	if _, err := s.verificationCodes.Upsert(ctx, userID, purpose, target, hashOTP(code), time.Now().Add(otpExpiry)); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if err := s.dispatchVerificationCode(ctx, purpose, target, code); err != nil {
		slog.Default().Error("verification code dispatch failed", "purpose", purpose, "error", err)
		return nil, apperror.ToGRPCStatus(fmt.Errorf("failed to send verification code, please try again: %w", apperror.ErrInternal))
	}

	return &authv1.StartVerificationResponse{ResendAfterSeconds: int32(otpResendCooldown.Seconds())}, nil
}

func (s *Service) dispatchVerificationCode(ctx context.Context, purpose repository.VerificationPurpose, target, code string) error {
	switch purpose {
	case repository.VerificationPurposePhone:
		return s.sms.SendVerificationCode(ctx, target, code)
	case repository.VerificationPurposePersonalEmail:
		return s.email.SendVerificationCode(ctx, target, code, email.PurposePersonalEmail)
	case repository.VerificationPurposeCorporateEmail:
		return s.email.SendVerificationCode(ctx, target, code, email.PurposeCorporateEmail)
	default:
		return fmt.Errorf("service: unknown verification purpose %q", purpose)
	}
}

// verifyAndConsumeCode checks a presented code against the pending row for
// (userID, purpose), enforcing expiry, target match, and the attempt cap,
// and deletes the row on success (Step C/D's 5-attempt cap; ADR-003's
// minimal-retention principle for the corporate-email case, applied
// uniformly to all three purposes here rather than as a special case).
func (s *Service) verifyAndConsumeCode(ctx context.Context, userID string, purpose repository.VerificationPurpose, target, code string) error {
	pending, err := s.verificationCodes.Get(ctx, userID, purpose)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return fmt.Errorf("no pending verification code, please request a new one: %w", apperror.ErrInvalidInput)
		}
		return err
	}

	if time.Now().After(pending.ExpiresAt) {
		_ = s.verificationCodes.Delete(ctx, userID, purpose)
		return fmt.Errorf("code expired, please request a new one: %w", apperror.ErrInvalidInput)
	}
	if pending.Target != target {
		return fmt.Errorf("target does not match the pending verification: %w", apperror.ErrInvalidInput)
	}
	if pending.Attempts >= otpMaxAttempts {
		_ = s.verificationCodes.Delete(ctx, userID, purpose)
		return fmt.Errorf("too many attempts, please request a new code: %w", apperror.ErrInvalidInput)
	}

	if !otpMatches(pending.CodeHash, code) {
		updated, incErr := s.verificationCodes.IncrementAttempts(ctx, userID, purpose)
		if incErr == nil && updated.Attempts >= otpMaxAttempts {
			_ = s.verificationCodes.Delete(ctx, userID, purpose)
		}
		return fmt.Errorf("invalid code: %w", apperror.ErrInvalidInput)
	}

	if err := s.verificationCodes.Delete(ctx, userID, purpose); err != nil {
		return err
	}
	return nil
}

// sessionForVerifiedUser issues a fresh access/refresh token pair for user
// — every verification-completing RPC reissues immediately (rather than
// waiting for the next natural refresh) so the caller doesn't show a stale
// trust level for up to 15 minutes right after an action that just changed
// it.
func (s *Service) sessionForVerifiedUser(ctx context.Context, user repository.User) (*authv1.SessionResponse, error) {
	session, err := s.issueSession(ctx, user)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return session, nil
}
