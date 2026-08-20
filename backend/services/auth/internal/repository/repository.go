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
//
// AgeConfirmedOver18/AgeConfirmedAt are ADR-014's 18+ self-attestation —
// deliberately no date of birth stored, matching ADR-003's minimal-
// retention spirit. LinkedInSub being empty is now a real, expected case
// (Level 0, federated/email-password-only account) rather than something
// that can't happen — see trustlevel.go's computeTrustLevel. PasswordHash
// is empty for any account that never used the email+password path
// (ADR-014 §2) — same "presence IS the signal" convention as every other
// field here.
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

	AgeConfirmedOver18 bool
	AgeConfirmedAt     *time.Time
	PasswordHash       string

	// RatingAverage/RatingCount are written by services/meetup (SubmitRating,
	// ADR-015, docs/02-domain/domain-model.md § Rating) — services/auth only
	// ever reads them, both services sharing one literal Postgres database.
	RatingAverage float64
	RatingCount   int
}

// NewUser is the input to UserRepository.Create. LinkedInSub is set for
// LinkedIn's own direct-signup path (CompleteLinkedInOnboarding, unchanged
// by ADR-014 — LinkedIn still creates accounts directly) and left "" for
// every other creation path (Apple/Google via ResolveOrCreateIdentity,
// email+password via SignUpOrRecoverWithEmail's new-account branch).
// AgeConfirmedOver18 must be true by the time Create is called for a new
// account — every signup path rejects false before reaching here.
type NewUser struct {
	LinkedInSub        string
	FullName           string
	ProfilePhotoURL    string
	Headline           string
	TrustLevel         int
	AgeConfirmedOver18 bool
}

// UserRepository is the persistence boundary for user records.
type UserRepository interface {
	// GetByLinkedInSub returns apperror.ErrNotFound (wrapped) if no user
	// has this LinkedIn sub.
	GetByLinkedInSub(ctx context.Context, linkedInSub string) (User, error)
	// GetByID returns apperror.ErrNotFound (wrapped) if no such user exists.
	GetByID(ctx context.Context, id string) (User, error)
	// GetByPersonalEmail returns apperror.ErrNotFound (wrapped) if no user
	// has this verified personal_email — SignUpOrRecoverWithEmail's
	// recovery-detection lookup (ADR-014 decision #3).
	GetByPersonalEmail(ctx context.Context, email string) (User, error)
	Create(ctx context.Context, u NewUser) (User, error)
	// SetPasswordHash sets password_hash on an existing user — used by
	// both a fresh email+password signup and SignUpOrRecoverWithEmail's
	// recovery path (setting a password on an existing account IS the
	// recovery mechanism, ADR-014 decision #3). hash is always an
	// already-computed argon2id hash, never the raw password.
	SetPasswordHash(ctx context.Context, userID, hash string) (User, error)

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
	// UpdateLinkedInSub links LinkedIn to an already-existing account
	// (ADR-014 — LinkedIn no longer creates accounts). Returns
	// apperror.ErrConflict (wrapped) if linkedInSub already belongs to a
	// different user — idx_users_linkedin_sub (migration 0001) is what
	// actually resolves that race, same pattern as UpdatePhoneNumber.
	UpdateLinkedInSub(ctx context.Context, userID, linkedInSub string, trustLevel int) (User, error)
}

// IdentityProvider mirrors the identity_provider Postgres enum (migration
// 0004) — Apple/Google only. LinkedIn is deliberately NOT a value here:
// its identity lives on users.linkedin_sub directly, both for direct
// signup (CompleteLinkedInOnboarding) and Profile-linking
// (LinkIdentityToUser's LinkedIn branch), so there is exactly one place
// to check "does this user have LinkedIn" regardless of when it was
// connected (ADR-014's Consequences).
type IdentityProvider string

const (
	IdentityProviderApple  IdentityProvider = "apple"
	IdentityProviderGoogle IdentityProvider = "google"
)

// UserIdentity is a linked federated-identity row (migration 0004) — the
// normalized "zero or more provider identities per user" shape ADR-014
// introduces for Apple/Google specifically. Linking Apple and/or Google to
// an account, in any combination, never raises trust level — they prove
// device/platform identity, not professional identity (ADR-014 §1).
type UserIdentity struct {
	ID       string
	UserID   string
	Provider IdentityProvider
	Subject  string
	Email    string
	LinkedAt time.Time
}

// UserIdentityRepository is the persistence boundary for linked federated
// identities (Apple/Google today).
type UserIdentityRepository interface {
	Insert(ctx context.Context, userID string, provider IdentityProvider, subject, email string) (UserIdentity, error)
	// GetByProviderSubject returns apperror.ErrNotFound (wrapped) if no
	// identity matches — the caller's signal to create a new Level 0
	// account rather than log in to an existing one.
	GetByProviderSubject(ctx context.Context, provider IdentityProvider, subject string) (UserIdentity, error)
	ListForUser(ctx context.Context, userID string) ([]UserIdentity, error)
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
// (migration 0002, extended by migration 0004) — one shared OTP mechanism
// used for all four purposes (backend/PLAN.md's addendum, Step C/D;
// ADR-014 decision #2 for email_signup), not four separate ones.
type VerificationPurpose string

const (
	VerificationPurposePhone          VerificationPurpose = "phone"
	VerificationPurposePersonalEmail  VerificationPurpose = "personal_email"
	VerificationPurposeCorporateEmail VerificationPurpose = "corporate_email"
	// VerificationPurposeEmailSignup is the one purpose with no user_id yet
	// at OTP-send time — the account may not exist at all (see
	// SignUpOrRecoverWithEmail's recovery path) — so rows for this purpose
	// are keyed by (purpose, target) instead of (user_id, purpose); see
	// VerificationCodeRepository's *ForSignup/*ByTarget methods below.
	VerificationPurposeEmailSignup VerificationPurpose = "email_signup"
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

	// UpsertForSignup/GetByTarget/IncrementAttemptsByTarget/DeleteByTarget
	// mirror Upsert/Get/IncrementAttempts/Delete exactly, except keyed by
	// (purpose, target) instead of (userID, purpose) — used only for
	// VerificationPurposeEmailSignup, where no user_id exists yet at
	// OTP-send time (migration 0004's idx_verification_codes_signup_target,
	// ADR-014 decision #2). purpose is always
	// VerificationPurposeEmailSignup today but is still passed explicitly
	// rather than hardcoded, matching the userID-keyed methods' shape.
	UpsertForSignup(ctx context.Context, purpose VerificationPurpose, target, codeHash string, expiresAt time.Time) (VerificationCode, error)
	// GetByTarget returns apperror.ErrNotFound (wrapped) if no pending code
	// exists for (purpose, target).
	GetByTarget(ctx context.Context, purpose VerificationPurpose, target string) (VerificationCode, error)
	IncrementAttemptsByTarget(ctx context.Context, purpose VerificationPurpose, target string) (VerificationCode, error)
	DeleteByTarget(ctx context.Context, purpose VerificationPurpose, target string) error
}
