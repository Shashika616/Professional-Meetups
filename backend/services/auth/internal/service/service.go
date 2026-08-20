// Package service implements the generated AuthServiceServer gRPC
// interface: LinkedIn OIDC exchange, user creation, JWT issuance, and
// refresh-token rotation (ADR-006, ADR-009, ADR-011).
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/professional-connections/backend/services/auth/internal/email"
	"github.com/professional-connections/backend/services/auth/internal/events"
	"github.com/professional-connections/backend/services/auth/internal/identity"
	"github.com/professional-connections/backend/services/auth/internal/linkedin"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/services/auth/internal/sms"
	"github.com/professional-connections/backend/shared/apperror"
	sharedjwt "github.com/professional-connections/backend/shared/jwt"
	authv1 "github.com/professional-connections/backend/shared/proto/auth/v1"
)

// RefreshTokenTTL is how long a refresh token stays valid before it must be
// rotated. Unlike shared/jwt.AccessTokenTTL (a hardcoded security
// constant), this is a product/UX parameter — how often a mobile user must
// eventually be prompted through LinkedIn again — so it lives here, not in
// shared/jwt.
const RefreshTokenTTL = 30 * 24 * time.Hour

// Service implements authv1.AuthServiceServer. Every dependency is passed
// explicitly via New — no framework, no globals.
type Service struct {
	authv1.UnimplementedAuthServiceServer

	users             repository.UserRepository
	identities        repository.UserIdentityRepository
	refreshTokens     repository.RefreshTokenRepository
	verificationCodes repository.VerificationCodeRepository
	linkedin          *linkedin.Client
	apple             identity.Provider
	google            identity.Provider
	signer            *sharedjwt.Signer
	events            events.Publisher
	email             email.EmailSender
	sms               sms.SmsSender
}

// New constructs a Service.
func New(
	users repository.UserRepository,
	identities repository.UserIdentityRepository,
	refreshTokens repository.RefreshTokenRepository,
	verificationCodes repository.VerificationCodeRepository,
	linkedinClient *linkedin.Client,
	appleProvider identity.Provider,
	googleProvider identity.Provider,
	signer *sharedjwt.Signer,
	publisher events.Publisher,
	emailSender email.EmailSender,
	smsSender sms.SmsSender,
) *Service {
	return &Service{
		users:             users,
		identities:        identities,
		refreshTokens:     refreshTokens,
		verificationCodes: verificationCodes,
		linkedin:          linkedinClient,
		apple:             appleProvider,
		google:            googleProvider,
		signer:            signer,
		events:            publisher,
		email:             emailSender,
		sms:               smsSender,
	}
}

// CompleteFederatedSignup creates or resolves a Level 0 account via Sign in
// with Apple or Google Sign-In (ADR-014). Unlike LinkedIn's flow, there is
// no server-to-server exchange here — id_token arrives already signed by
// the provider, straight from their native SDK, and is verified purely
// cryptographically (internal/identity) before ResolveOrCreateIdentity (the
// one shared resolve-or-create function every federated login uses,
// identity_resolution.go) ever sees it.
func (s *Service) CompleteFederatedSignup(
	ctx context.Context, req *authv1.CompleteFederatedSignupRequest,
) (*authv1.SessionResponse, error) {
	provider, providerName, err := s.federatedProvider(req.GetProvider())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	verified, err := provider.Verify(ctx, req.GetIdToken())
	if err != nil {
		// err may embed provider-specific verification detail — logged here
		// with full detail, but never handed to the client, same discipline
		// LinkedIn's own exchange errors already use below.
		slog.Default().Error("federated id_token verification failed", "provider", providerName, "error", err)
		return nil, apperror.ToGRPCStatus(fmt.Errorf("sign-in failed, please try again: %w", apperror.ErrInvalidInput))
	}
	// The id_token itself is discarded after this call — never persisted,
	// same "verify, extract, discard" discipline as LinkedIn's access
	// token (ADR-011/ADR-014).

	// full_name may be "" here (Apple in particular only ever includes a
	// name on a user's very first authorization with this app) — the
	// client already knows to render a fallback ("Member"), same as an
	// unresolved-profile HomePage load today.
	user, isNewUser, err := s.ResolveOrCreateIdentity(ctx, providerName, verified.Subject, verified.Email, verified.Name, "", req.GetAgeConfirmedOver_18())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return s.finishSignupSession(ctx, user, isNewUser)
}

// federatedProvider maps the wire enum to this Service's constructed
// Provider and the FederatedProvider to resolve/create against — the only
// two values CompleteFederatedSignup ever accepts (LinkedIn direct signup
// is CompleteLinkedInOnboarding, not this RPC).
func (s *Service) federatedProvider(p authv1.IdentityProviderProto) (identity.Provider, FederatedProvider, error) {
	switch p {
	case authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE:
		return s.apple, FederatedProviderApple, nil
	case authv1.IdentityProviderProto_IDENTITY_PROVIDER_GOOGLE:
		return s.google, FederatedProviderGoogle, nil
	default:
		return nil, "", fmt.Errorf("unsupported identity provider for account creation: %v: %w", p, apperror.ErrInvalidInput)
	}
}

// CompleteLinkedInOnboarding creates or resolves a Level 1 account directly
// via LinkedIn — unchanged in behavior from ADR-011 (still unauthenticated,
// still the one path that grants Level 1 immediately), except
// age_confirmed_over_18 is now required and rejected server-side if false,
// same as every other signup path (ADR-014). Internally calls the same
// ResolveOrCreateIdentity every federated login uses, not a separate
// lookup.
func (s *Service) CompleteLinkedInOnboarding(
	ctx context.Context, req *authv1.CompleteLinkedInOnboardingRequest,
) (*authv1.SessionResponse, error) {
	token, err := s.linkedin.ExchangeCode(ctx, req.GetAuthorizationCode(), req.GetRedirectUri())
	if err != nil {
		// err embeds LinkedIn's raw upstream response body (linkedin.Client's
		// error strings) — logged here with full detail, but never handed to
		// the client: that body is untrusted-boundary-crossing implementation
		// detail, not something safe to echo back over the REST API.
		slog.Default().Error("linkedin code exchange failed", "error", err)
		return nil, apperror.ToGRPCStatus(fmt.Errorf("linkedin sign-in failed, please try again: %w", apperror.ErrInvalidInput))
	}

	// token is discarded after this call — never persisted (ADR-011).
	info, err := s.linkedin.FetchUserInfo(ctx, token)
	if err != nil {
		slog.Default().Error("linkedin userinfo fetch failed", "error", err)
		return nil, apperror.ToGRPCStatus(fmt.Errorf("linkedin sign-in failed, please try again: %w", apperror.ErrInvalidInput))
	}

	user, isNewUser, err := s.ResolveOrCreateIdentity(ctx, FederatedProviderLinkedIn, info.Sub, "", info.Name, info.Picture, req.GetAgeConfirmedOver_18())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return s.finishSignupSession(ctx, user, isNewUser)
}

// finishSignupSession issues a session for a just-resolved-or-created user
// and, for a brand-new account only, publishes the onboarding event — the
// shared tail end of every signup RPC (Apple/Google, LinkedIn, email+
// password) so publish-on-create logic lives in exactly one place.
func (s *Service) finishSignupSession(ctx context.Context, user repository.User, isNewUser bool) (*authv1.SessionResponse, error) {
	session, err := s.issueSession(ctx, user)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	if isNewUser {
		if err := s.events.PublishUserOnboarded(ctx, user.ID, user.TrustLevel); err != nil {
			return nil, apperror.ToGRPCStatus(fmt.Errorf("publish user.onboarded: %w: %w", apperror.ErrInternal, err))
		}
	}
	session.IsNewUser = isNewUser
	return session, nil
}

// LinkIdentity links an additional identity to the caller's
// already-authenticated account (ADR-014's Profile "Connect LinkedIn" flow,
// or a future "add Apple/Google as backup sign-in"). Dispatches to the
// LinkedIn code-exchange or Apple/Google id_token-verification path
// depending on req.GetProvider(), then calls the one shared
// LinkIdentityToUser (identity_resolution.go), which hard-rejects on a
// cross-user collision.
func (s *Service) LinkIdentity(
	ctx context.Context, req *authv1.LinkIdentityRequest,
) (*authv1.SessionResponse, error) {
	var provider FederatedProvider
	var subject string

	switch req.GetProvider() {
	case authv1.IdentityProviderProto_IDENTITY_PROVIDER_LINKEDIN:
		token, err := s.linkedin.ExchangeCode(ctx, req.GetAuthorizationCode(), req.GetRedirectUri())
		if err != nil {
			slog.Default().Error("linkedin code exchange failed", "error", err)
			return nil, apperror.ToGRPCStatus(fmt.Errorf("linkedin sign-in failed, please try again: %w", apperror.ErrInvalidInput))
		}
		// token is discarded after this call — never persisted (ADR-011).
		info, err := s.linkedin.FetchUserInfo(ctx, token)
		if err != nil {
			slog.Default().Error("linkedin userinfo fetch failed", "error", err)
			return nil, apperror.ToGRPCStatus(fmt.Errorf("linkedin sign-in failed, please try again: %w", apperror.ErrInvalidInput))
		}
		provider, subject = FederatedProviderLinkedIn, info.Sub

	case authv1.IdentityProviderProto_IDENTITY_PROVIDER_APPLE, authv1.IdentityProviderProto_IDENTITY_PROVIDER_GOOGLE:
		idProvider, providerName, err := s.federatedProvider(req.GetProvider())
		if err != nil {
			return nil, apperror.ToGRPCStatus(err)
		}
		verified, err := idProvider.Verify(ctx, req.GetIdToken())
		if err != nil {
			slog.Default().Error("federated id_token verification failed", "provider", providerName, "error", err)
			return nil, apperror.ToGRPCStatus(fmt.Errorf("sign-in failed, please try again: %w", apperror.ErrInvalidInput))
		}
		provider, subject = providerName, verified.Subject

	default:
		return nil, apperror.ToGRPCStatus(fmt.Errorf("unsupported identity provider for LinkIdentity: %v: %w", req.GetProvider(), apperror.ErrInvalidInput))
	}

	if err := s.LinkIdentityToUser(ctx, req.GetUserId(), provider, subject); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	user, err := s.users.GetByID(ctx, req.GetUserId())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return s.sessionForVerifiedUser(ctx, user)
}

// StartEmailSignup sends an OTP to email as the first step of the
// email+password signup flow (ADR-014 decision #2). Unauthenticated — there
// is no user_id yet (the account may never even be created, see
// SignUpOrRecoverWithEmail's recovery path) — so this cannot reuse
// startVerificationRPC's userID-keyed Upsert/Get; it uses the target-keyed
// verificationCodes methods instead (migration 0004).
func (s *Service) StartEmailSignup(ctx context.Context, req *authv1.StartVerificationRequest) (*authv1.StartVerificationResponse, error) {
	if req.GetPurpose() != authv1.VerificationPurpose_VERIFICATION_PURPOSE_EMAIL_SIGNUP {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("purpose mismatch for StartEmailSignup: %w", apperror.ErrInvalidInput))
	}
	target := req.GetTarget()
	if target == "" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("target is required: %w", apperror.ErrInvalidInput))
	}

	existing, err := s.verificationCodes.GetByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
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

	if _, err := s.verificationCodes.UpsertForSignup(ctx, repository.VerificationPurposeEmailSignup, target, hashOTP(code), time.Now().Add(otpExpiry)); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	if err := s.email.SendVerificationCode(ctx, target, code, email.PurposePersonalEmail); err != nil {
		slog.Default().Error("verification code dispatch failed", "purpose", repository.VerificationPurposeEmailSignup, "error", err)
		return nil, apperror.ToGRPCStatus(fmt.Errorf("failed to send verification code, please try again: %w", apperror.ErrInternal))
	}

	return &authv1.StartVerificationResponse{ResendAfterSeconds: int32(otpResendCooldown.Seconds())}, nil
}

// CompleteEmailSignup verifies the OTP sent by StartEmailSignup, hashes the
// password (argon2id, password.go), then calls SignUpOrRecoverWithEmail —
// which may create a new Level 0 user or recover an existing one (see that
// function's doc comment).
func (s *Service) CompleteEmailSignup(ctx context.Context, req *authv1.CompleteEmailSignupRequest) (*authv1.SessionResponse, error) {
	if err := s.verifyAndConsumeSignupCode(ctx, req.GetEmail(), req.GetCode()); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	hash, err := hashPassword(req.GetPassword())
	if err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("%w: %w", apperror.ErrInternal, err))
	}

	user, isNewUser, err := s.SignUpOrRecoverWithEmail(ctx, req.GetEmail(), hash, req.GetAgeConfirmedOver_18())
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	return s.finishSignupSession(ctx, user, isNewUser)
}

// verifyAndConsumeSignupCode mirrors verifyAndConsumeCode exactly, except
// keyed by (purpose, target) instead of (userID, purpose) — see
// StartEmailSignup's doc comment for why. Unlike verifyAndConsumeCode,
// there's no separate "target matches" check: GetByTarget already looks
// the row up BY target, so a mismatch can't occur here the way it can for
// the userID-keyed methods.
func (s *Service) verifyAndConsumeSignupCode(ctx context.Context, target, code string) error {
	pending, err := s.verificationCodes.GetByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return fmt.Errorf("no pending verification code, please request a new one: %w", apperror.ErrInvalidInput)
		}
		return err
	}

	if time.Now().After(pending.ExpiresAt) {
		_ = s.verificationCodes.DeleteByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
		return fmt.Errorf("code expired, please request a new one: %w", apperror.ErrInvalidInput)
	}
	if pending.Attempts >= otpMaxAttempts {
		_ = s.verificationCodes.DeleteByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
		return fmt.Errorf("too many attempts, please request a new code: %w", apperror.ErrInvalidInput)
	}

	if !otpMatches(pending.CodeHash, code) {
		updated, incErr := s.verificationCodes.IncrementAttemptsByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
		if incErr == nil && updated.Attempts >= otpMaxAttempts {
			_ = s.verificationCodes.DeleteByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
		}
		return fmt.Errorf("invalid code: %w", apperror.ErrInvalidInput)
	}

	return s.verificationCodes.DeleteByTarget(ctx, repository.VerificationPurposeEmailSignup, target)
}

// LoginWithPassword returns the same generic "invalid email or password"
// error whether the email doesn't exist, has no password set (a
// federated/LinkedIn-only account never used this path), or the password
// is wrong — an account-enumeration-safe pattern, same discipline as every
// other lookup-by-target RPC in this service.
//
// verifyPassword always runs, against dummyPasswordHash when the account
// has no real one (ADR-016) — a prior version short-circuited past
// verifyPassword entirely for a federated-only account, so it returned
// near-instantly while a real wrong-password attempt paid the full
// argon2id cost first: same error message, different timing, a measurable
// side channel that undercut the "enumeration-safe" claim above. Running
// the comparison unconditionally, then separately checking PasswordHash's
// presence, keeps both paths' cost identical.
func (s *Service) LoginWithPassword(ctx context.Context, req *authv1.LoginWithPasswordRequest) (*authv1.SessionResponse, error) {
	user, err := s.users.GetByPersonalEmail(ctx, req.GetEmail())
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			return nil, apperror.ToGRPCStatus(fmt.Errorf("invalid email or password: %w", apperror.ErrUnauthorized))
		}
		return nil, apperror.ToGRPCStatus(err)
	}
	hash := user.PasswordHash
	if hash == "" {
		hash = dummyPasswordHash
	}
	if !verifyPassword(hash, req.GetPassword()) || user.PasswordHash == "" {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("invalid email or password: %w", apperror.ErrUnauthorized))
	}
	return s.sessionForVerifiedUser(ctx, user)
}

// RefreshSession rotates a refresh token for a new access/refresh token
// pair. The presented token is invalidated whether or not it was valid to
// begin with (ADR-009).
func (s *Service) RefreshSession(
	ctx context.Context, req *authv1.RefreshSessionRequest,
) (*authv1.SessionResponse, error) {
	old, err := s.refreshTokens.FindByHash(ctx, hashToken(req.GetRefreshToken()))
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	// A replayed, already-rotated (or revoked) refresh token is a possible
	// theft signal (ADR-009) — reject rather than silently accept.
	if old.RevokedAt != nil || old.ReplacedBy != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("refresh token already used: %w", apperror.ErrUnauthorized))
	}
	if time.Now().After(old.ExpiresAt) {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("refresh token expired: %w", apperror.ErrUnauthorized))
	}

	user, err := s.users.GetByID(ctx, old.UserID)
	if err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	newRawToken, newHash, err := newRefreshToken()
	if err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("%w: %w", apperror.ErrInternal, err))
	}

	if _, err := s.refreshTokens.Rotate(ctx, old.ID, newHash, time.Now().Add(RefreshTokenTTL)); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}

	accessToken, err := s.signer.Sign(sharedjwt.Claims{UserID: user.ID, TrustLevel: user.TrustLevel})
	if err != nil {
		return nil, apperror.ToGRPCStatus(fmt.Errorf("%w: %w", apperror.ErrInternal, err))
	}

	return &authv1.SessionResponse{
		UserId:                      user.ID,
		AccessToken:                 accessToken,
		RefreshToken:                newRawToken,
		AccessTokenExpiresInSeconds: int64(sharedjwt.AccessTokenTTL.Seconds()),
		FullName:                    user.FullName,
		ProfilePhotoUrl:             user.ProfilePhotoURL,
	}, nil
}

// RevokeSession revokes a refresh token (logout). Idempotent — revoking an
// already-revoked or unknown token is not an error.
func (s *Service) RevokeSession(
	ctx context.Context, req *authv1.RevokeSessionRequest,
) (*authv1.RevokeSessionResponse, error) {
	if err := s.refreshTokens.Revoke(ctx, hashToken(req.GetRefreshToken())); err != nil {
		return nil, apperror.ToGRPCStatus(err)
	}
	return &authv1.RevokeSessionResponse{Success: true}, nil
}

// issueSession mints and persists a fresh access/refresh token pair for
// user.
func (s *Service) issueSession(ctx context.Context, user repository.User) (*authv1.SessionResponse, error) {
	rawToken, hash, err := newRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperror.ErrInternal, err)
	}

	if _, err := s.refreshTokens.Create(ctx, user.ID, hash, time.Now().Add(RefreshTokenTTL)); err != nil {
		return nil, err
	}

	accessToken, err := s.signer.Sign(sharedjwt.Claims{UserID: user.ID, TrustLevel: user.TrustLevel})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", apperror.ErrInternal, err)
	}

	return &authv1.SessionResponse{
		UserId:                      user.ID,
		AccessToken:                 accessToken,
		RefreshToken:                rawToken,
		AccessTokenExpiresInSeconds: int64(sharedjwt.AccessTokenTTL.Seconds()),
		FullName:                    user.FullName,
		ProfilePhotoUrl:             user.ProfilePhotoURL,
	}, nil
}

// newRefreshToken generates a new random refresh token, returning both the
// raw value (returned to the client exactly once) and its SHA-256 hash —
// the only thing ever persisted (ADR-009).
func newRefreshToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
