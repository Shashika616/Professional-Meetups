// Package repository is the persistence boundary for services/auth:
// interfaces first, Postgres implementation second, mirroring the pattern
// already established on the Flutter side (AuthService/MatchingService
// abstract interfaces + Mock* implementations — see the root CLAUDE.md's
// "Service-contract pattern"). internal/service/ depends on these
// interfaces, never directly on Postgres or sqlcgen.
package repository

import (
	"context"
	"time"
)

// AccountStatus mirrors the account_status Postgres enum (db/migrations).
type AccountStatus string

const (
	AccountStatusActive      AccountStatus = "active"
	AccountStatusDeactivated AccountStatus = "deactivated"
	AccountStatusDeleted     AccountStatus = "deleted"
)

// User is the Level-1a-only user record (ADR-011) — no phone, personal
// email, personal details, corporate email, or KYC fields; those arrive in
// their own migration when that slice is built.
type User struct {
	ID              string
	LinkedInSub     string
	FullName        string
	ProfilePhotoURL string
	Headline        string
	TrustLevel      int
	AccountStatus   AccountStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewUser is the input to UserRepository.Create.
type NewUser struct {
	LinkedInSub     string
	FullName        string
	ProfilePhotoURL string
	Headline        string
	TrustLevel      int
}

// UserRepository is the persistence boundary for user records.
type UserRepository interface {
	// GetByLinkedInSub returns apperror.ErrNotFound (wrapped) if no user
	// has this LinkedIn sub.
	GetByLinkedInSub(ctx context.Context, linkedInSub string) (User, error)
	// GetByID returns apperror.ErrNotFound (wrapped) if no such user exists.
	GetByID(ctx context.Context, id string) (User, error)
	Create(ctx context.Context, u NewUser) (User, error)
}

// RefreshToken is a refresh-token row. TokenHash is always the SHA-256 hash
// of the raw token, hex-encoded — the raw token itself is never stored
// (ADR-009) and this type has no field for it.
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
}

// RefreshTokenRepository is the persistence boundary for refresh-token
// rotation state (ADR-009).
type RefreshTokenRepository interface {
	Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (RefreshToken, error)
	// FindByHash returns apperror.ErrNotFound (wrapped) if tokenHash is
	// unknown.
	FindByHash(ctx context.Context, tokenHash string) (RefreshToken, error)
	// Rotate marks the row identified by oldID as replaced and inserts a
	// new refresh-token row, in a single transaction, returning the new
	// row's ID. Returns apperror.ErrNotFound (wrapped) if oldID is unknown.
	Rotate(ctx context.Context, oldID, newTokenHash string, newExpiresAt time.Time) (newID string, err error)
	// Revoke is idempotent — revoking an already-revoked or unknown token
	// is not an error (mirrors the gateway's /v1/auth/logout contract).
	Revoke(ctx context.Context, tokenHash string) error
}
