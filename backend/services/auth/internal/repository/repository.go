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

// User is the user record, including Level 2/3 verification fields
// (ADR-012, backend/PLAN.md's matching addendum). Presence of PhoneNumber/
// PersonalEmail/LegalName/Address IS the "set" signal for each, same
// pattern as LinkedInSub — WorkEmailVerified is the one deliberate
// exception, kept as an explicit bool because ADR-012 names it as its own
// field. CompanyDomain is never the raw corporate email address (ADR-003).
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

	PhoneNumber         string
	PersonalEmail       string
	LegalName           string
	Address             string
	CompanyDomain       string
	WorkEmailVerified   bool
	WorkEmailVerifiedAt *time.Time
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

	// UpdatePhoneNumber/UpdatePersonalEmail return apperror.ErrConflict
	// (wrapped) if phoneNumber/personalEmail is already verified on a
	// different account — the UNIQUE constraint (migration 0002) is what
	// actually resolves the two-users-race-for-the-same-target case
	// (backend/PLAN.md's addendum, Step F); trustLevel is computed by the
	// caller (computeTrustLevel) and written in the same statement so the
	// row is never left with a stale value between the field write and a
	// separate recompute step.
	UpdatePhoneNumber(ctx context.Context, userID, phoneNumber string, trustLevel int) (User, error)
	UpdatePersonalEmail(ctx context.Context, userID, personalEmail string, trustLevel int) (User, error)
	UpdatePersonalDetails(ctx context.Context, userID, legalName, address string, trustLevel int) (User, error)
	UpdateWorkEmailVerified(ctx context.Context, userID, companyDomain string, verified bool, verifiedAt time.Time, trustLevel int) (User, error)
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

// VerificationPurpose mirrors the verification_purpose Postgres enum
// (migration 0002) — one shared OTP mechanism used for all three purposes
// (backend/PLAN.md's addendum, Step C/D), not three separate ones.
type VerificationPurpose string

const (
	VerificationPurposePhone          VerificationPurpose = "phone"
	VerificationPurposePersonalEmail  VerificationPurpose = "personal_email"
	VerificationPurposeCorporateEmail VerificationPurpose = "corporate_email"
)

// VerificationCode is a pending OTP row. CodeHash is always the SHA-256
// hash of the actual 6-digit code, hex-encoded — mirrors
// RefreshToken.TokenHash, the raw code is never stored.
type VerificationCode struct {
	ID        string
	UserID    string
	Purpose   VerificationPurpose
	Target    string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

// VerificationCodeRepository is the persistence boundary for pending OTP
// state, shared across phone/personal-email/corporate-email verification.
type VerificationCodeRepository interface {
	// Upsert writes a fresh code for (userID, purpose), overwriting any
	// existing pending code for the same pair (one pending code per
	// user+purpose — migration 0002's UNIQUE constraint) rather than
	// stacking a second one. Resets attempts to 0 and created_at to now,
	// which is what restarts the resend-cooldown window.
	Upsert(ctx context.Context, userID string, purpose VerificationPurpose, target, codeHash string, expiresAt time.Time) (VerificationCode, error)
	// Get returns apperror.ErrNotFound (wrapped) if no pending code exists
	// for (userID, purpose).
	Get(ctx context.Context, userID string, purpose VerificationPurpose) (VerificationCode, error)
	// IncrementAttempts returns the row's new attempt count after
	// incrementing — callers compare this against the 5-attempt cap.
	IncrementAttempts(ctx context.Context, userID string, purpose VerificationPurpose) (VerificationCode, error)
	// Delete removes the pending code — called on successful verification
	// (so the raw target, kept only transiently here, doesn't linger; the
	// corporate-email case's ADR-003 minimal-retention requirement) and can
	// also be used to force a fresh code after the attempt cap is hit.
	Delete(ctx context.Context, userID string, purpose VerificationPurpose) error
}
