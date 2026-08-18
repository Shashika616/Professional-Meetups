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
	refreshTokens     repository.RefreshTokenRepository
	verificationCodes repository.VerificationCodeRepository
	linkedin          *linkedin.Client
	signer            *sharedjwt.Signer
	events            events.Publisher
	email             email.EmailSender
	sms               sms.SmsSender
}

// New constructs a Service.
func New(
	users repository.UserRepository,
	refreshTokens repository.RefreshTokenRepository,
	verificationCodes repository.VerificationCodeRepository,
	linkedinClient *linkedin.Client,
	signer *sharedjwt.Signer,
	publisher events.Publisher,
	emailSender email.EmailSender,
	smsSender sms.SmsSender,
) *Service {
	return &Service{
		users:             users,
		refreshTokens:     refreshTokens,
		verificationCodes: verificationCodes,
		linkedin:          linkedinClient,
		signer:            signer,
		events:            publisher,
		email:             emailSender,
		sms:               smsSender,
	}
}

// CompleteLinkedInOnboarding exchanges a LinkedIn authorization code for a
// session, creating the user record on first login.
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

	isNewUser := false
	user, err := s.users.GetByLinkedInSub(ctx, info.Sub)
	if err != nil {
		if !errors.Is(err, apperror.ErrNotFound) {
			return nil, apperror.ToGRPCStatus(err)
		}

		user, err = s.users.Create(ctx, repository.NewUser{
			LinkedInSub:     info.Sub,
			FullName:        info.Name,
			ProfilePhotoURL: info.Picture,
			TrustLevel:      1,
		})
		if err != nil {
			return nil, apperror.ToGRPCStatus(err)
		}
		isNewUser = true
	}

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
