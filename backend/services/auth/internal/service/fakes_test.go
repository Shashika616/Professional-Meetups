package service

import (
	"context"
	"fmt"
	"time"

	"github.com/professional-connections/backend/services/auth/internal/email"
	"github.com/professional-connections/backend/services/auth/internal/identity"
	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
)

// fakeIdentityProvider stands in for AppleProvider/GoogleProvider — a
// fixed VerifiedIdentity per valid id_token string, or an error for
// anything else. Real signature/issuer/audience/expiry verification is
// internal/identity's own concern (identity_test.go); this fake is only
// about exercising CompleteFederatedSignup's account-creation/login
// branching without a real JWKS round trip.
type fakeIdentityProvider struct {
	validTokens map[string]identity.VerifiedIdentity
	err         error
}

func (f *fakeIdentityProvider) Verify(_ context.Context, idToken string) (identity.VerifiedIdentity, error) {
	if f.err != nil {
		return identity.VerifiedIdentity{}, f.err
	}
	v, ok := f.validTokens[idToken]
	if !ok {
		return identity.VerifiedIdentity{}, fmt.Errorf("fake: invalid id_token")
	}
	return v, nil
}

type fakeUserRepository struct {
	byLinkedInSub map[string]repository.User
	byID          map[string]repository.User
	createErr     error
	createCalls   []repository.NewUser
	nextID        int
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		byLinkedInSub: map[string]repository.User{},
		byID:          map[string]repository.User{},
	}
}

func (f *fakeUserRepository) GetByLinkedInSub(_ context.Context, linkedInSub string) (repository.User, error) {
	u, ok := f.byLinkedInSub[linkedInSub]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return u, nil
}

func (f *fakeUserRepository) GetByID(_ context.Context, id string) (repository.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return u, nil
}

// GetByPersonalEmail linear-scans byID — fine at fake-repository/test
// scale, mirrors GetUserByPersonalEmail's real "personal_email presence IS
// verified" semantics with no separate boolean to check.
func (f *fakeUserRepository) GetByPersonalEmail(_ context.Context, email string) (repository.User, error) {
	for _, u := range f.byID {
		if u.PersonalEmail == email {
			return u, nil
		}
	}
	return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
}

func (f *fakeUserRepository) SetPasswordHash(_ context.Context, userID, hash string) (repository.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.PasswordHash = hash
	f.save(u)
	return u, nil
}

func (f *fakeUserRepository) Create(_ context.Context, u repository.NewUser) (repository.User, error) {
	if f.createErr != nil {
		return repository.User{}, f.createErr
	}
	f.createCalls = append(f.createCalls, u)
	f.nextID++
	created := repository.User{
		ID:                 fmt.Sprintf("user-%d", f.nextID),
		LinkedInSub:        u.LinkedInSub,
		FullName:           u.FullName,
		ProfilePhotoURL:    u.ProfilePhotoURL,
		TrustLevel:         u.TrustLevel,
		AccountStatus:      repository.AccountStatusActive,
		AgeConfirmedOver18: u.AgeConfirmedOver18,
	}
	// Mirrors the real CreateUser query: age_confirmed_at is set only when
	// age_confirmed_over_18 is true, never backdated, never set for false.
	if u.AgeConfirmedOver18 {
		now := time.Now()
		created.AgeConfirmedAt = &now
	}
	f.byLinkedInSub[u.LinkedInSub] = created
	f.byID[created.ID] = created
	return created, nil
}

// UpdatePhoneNumber/UpdatePersonalEmail simulate migration 0002's UNIQUE
// constraint — a linear scan for a conflicting value on a different user is
// fine at fake-repository scale and is what lets
// TestVerifyPhoneCode_ConflictOnAlreadyVerifiedNumber (etc.) exercise the
// same race-condition mapping the real Postgres repository provides via
// pgErr.Code == "23505".
func (f *fakeUserRepository) UpdatePhoneNumber(_ context.Context, userID, phoneNumber string, trustLevel int) (repository.User, error) {
	for id, u := range f.byID {
		if id != userID && u.PhoneNumber == phoneNumber {
			return repository.User{}, fmt.Errorf("fake: phone number already verified on a different account: %w", apperror.ErrConflict)
		}
	}
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.PhoneNumber = phoneNumber
	u.TrustLevel = trustLevel
	f.save(u)
	return u, nil
}

func (f *fakeUserRepository) UpdatePersonalEmail(_ context.Context, userID, personalEmail string, trustLevel int) (repository.User, error) {
	for id, u := range f.byID {
		if id != userID && u.PersonalEmail == personalEmail {
			return repository.User{}, fmt.Errorf("fake: personal email already verified on a different account: %w", apperror.ErrConflict)
		}
	}
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.PersonalEmail = personalEmail
	u.TrustLevel = trustLevel
	f.save(u)
	return u, nil
}

func (f *fakeUserRepository) UpdatePersonalDetails(_ context.Context, userID, legalName, address string, trustLevel int) (repository.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.LegalName = legalName
	u.Address = address
	u.TrustLevel = trustLevel
	f.save(u)
	return u, nil
}

func (f *fakeUserRepository) UpdateWorkEmailVerified(_ context.Context, userID, companyDomain string, verified bool, verifiedAt time.Time, trustLevel int) (repository.User, error) {
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.CompanyDomain = companyDomain
	u.WorkEmailVerified = verified
	u.WorkEmailVerifiedAt = &verifiedAt
	u.TrustLevel = trustLevel
	f.save(u)
	return u, nil
}

// UpdateLinkedInSub simulates idx_users_linkedin_sub (migration 0001) the
// same way UpdatePhoneNumber/UpdatePersonalEmail simulate their own unique
// constraints above — a linear scan for a conflicting value on a different
// user, exercising the same real-Postgres-mapped ErrConflict path.
func (f *fakeUserRepository) UpdateLinkedInSub(_ context.Context, userID, linkedInSub string, trustLevel int) (repository.User, error) {
	for id, u := range f.byID {
		if id != userID && u.LinkedInSub == linkedInSub {
			return repository.User{}, fmt.Errorf("fake: linkedin account already linked to a different user: %w", apperror.ErrConflict)
		}
	}
	u, ok := f.byID[userID]
	if !ok {
		return repository.User{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	u.LinkedInSub = linkedInSub
	u.TrustLevel = trustLevel
	f.save(u)
	return u, nil
}

func (f *fakeUserRepository) save(u repository.User) {
	f.byID[u.ID] = u
	if u.LinkedInSub != "" {
		f.byLinkedInSub[u.LinkedInSub] = u
	}
}

// fakeUserIdentityRepository keys rows by (provider, subject), mirroring
// migration 0004's UNIQUE(provider, subject) — an Insert for an existing
// key returns ErrConflict rather than silently overwriting.
type fakeUserIdentityRepository struct {
	byProviderSubject map[string]repository.UserIdentity
	nextID            int
}

func newFakeUserIdentityRepository() *fakeUserIdentityRepository {
	return &fakeUserIdentityRepository{byProviderSubject: map[string]repository.UserIdentity{}}
}

func identityKey(provider repository.IdentityProvider, subject string) string {
	return string(provider) + "|" + subject
}

func (f *fakeUserIdentityRepository) Insert(
	_ context.Context, userID string, provider repository.IdentityProvider, subject, email string,
) (repository.UserIdentity, error) {
	key := identityKey(provider, subject)
	if _, exists := f.byProviderSubject[key]; exists {
		return repository.UserIdentity{}, fmt.Errorf("fake: %s identity already linked: %w", provider, apperror.ErrConflict)
	}
	f.nextID++
	row := repository.UserIdentity{
		ID:       fmt.Sprintf("identity-%d", f.nextID),
		UserID:   userID,
		Provider: provider,
		Subject:  subject,
		Email:    email,
		LinkedAt: time.Now(),
	}
	f.byProviderSubject[key] = row
	return row, nil
}

func (f *fakeUserIdentityRepository) GetByProviderSubject(
	_ context.Context, provider repository.IdentityProvider, subject string,
) (repository.UserIdentity, error) {
	row, ok := f.byProviderSubject[identityKey(provider, subject)]
	if !ok {
		return repository.UserIdentity{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return row, nil
}

func (f *fakeUserIdentityRepository) ListForUser(_ context.Context, userID string) ([]repository.UserIdentity, error) {
	var out []repository.UserIdentity
	for _, row := range f.byProviderSubject {
		if row.UserID == userID {
			out = append(out, row)
		}
	}
	return out, nil
}

type fakeRefreshTokenRepository struct {
	byHash    map[string]repository.RefreshToken
	byID      map[string]repository.RefreshToken
	nextID    int
	createErr error
	rotateErr error
}

func newFakeRefreshTokenRepository() *fakeRefreshTokenRepository {
	return &fakeRefreshTokenRepository{
		byHash: map[string]repository.RefreshToken{},
		byID:   map[string]repository.RefreshToken{},
	}
}

func (f *fakeRefreshTokenRepository) Create(_ context.Context, userID, tokenHash string, expiresAt time.Time) (repository.RefreshToken, error) {
	if f.createErr != nil {
		return repository.RefreshToken{}, f.createErr
	}
	f.nextID++
	rt := repository.RefreshToken{
		ID:        fmt.Sprintf("rt-%d", f.nextID),
		UserID:    userID,
		TokenHash: tokenHash,
		IssuedAt:  time.Now(),
		ExpiresAt: expiresAt,
	}
	f.byHash[tokenHash] = rt
	f.byID[rt.ID] = rt
	return rt, nil
}

func (f *fakeRefreshTokenRepository) FindByHash(_ context.Context, tokenHash string) (repository.RefreshToken, error) {
	rt, ok := f.byHash[tokenHash]
	if !ok {
		return repository.RefreshToken{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return rt, nil
}

func (f *fakeRefreshTokenRepository) Rotate(_ context.Context, oldID, newTokenHash string, newExpiresAt time.Time) (string, error) {
	if f.rotateErr != nil {
		return "", f.rotateErr
	}
	old, ok := f.byID[oldID]
	if !ok {
		return "", fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}

	f.nextID++
	newID := fmt.Sprintf("rt-%d", f.nextID)
	newRT := repository.RefreshToken{
		ID:        newID,
		UserID:    old.UserID,
		TokenHash: newTokenHash,
		IssuedAt:  time.Now(),
		ExpiresAt: newExpiresAt,
	}

	old.ReplacedBy = &newID
	f.byID[oldID] = old
	f.byHash[old.TokenHash] = old
	f.byID[newID] = newRT
	f.byHash[newTokenHash] = newRT

	return newID, nil
}

func (f *fakeRefreshTokenRepository) Revoke(_ context.Context, tokenHash string) error {
	rt, ok := f.byHash[tokenHash]
	if !ok {
		return nil // idempotent: unknown token is not an error
	}
	now := time.Now()
	rt.RevokedAt = &now
	f.byHash[tokenHash] = rt
	f.byID[rt.ID] = rt
	return nil
}

// fakeVerificationCodeRepository keys rows by (userID, purpose), mirroring
// migration 0002's UNIQUE(user_id, purpose) — an Upsert for an existing key
// overwrites rather than adding a second row.
type fakeVerificationCodeRepository struct {
	rows       map[string]repository.VerificationCode
	targetRows map[string]repository.VerificationCode
}

func newFakeVerificationCodeRepository() *fakeVerificationCodeRepository {
	return &fakeVerificationCodeRepository{
		rows:       map[string]repository.VerificationCode{},
		targetRows: map[string]repository.VerificationCode{},
	}
}

func verificationCodeKey(userID string, purpose repository.VerificationPurpose) string {
	return userID + "|" + string(purpose)
}

func (f *fakeVerificationCodeRepository) Upsert(
	_ context.Context, userID string, purpose repository.VerificationPurpose, target, codeHash string, expiresAt time.Time,
) (repository.VerificationCode, error) {
	row := repository.VerificationCode{
		ID:        verificationCodeKey(userID, purpose),
		UserID:    userID,
		Purpose:   purpose,
		Target:    target,
		CodeHash:  codeHash,
		Attempts:  0,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	f.rows[verificationCodeKey(userID, purpose)] = row
	return row, nil
}

func (f *fakeVerificationCodeRepository) Get(_ context.Context, userID string, purpose repository.VerificationPurpose) (repository.VerificationCode, error) {
	row, ok := f.rows[verificationCodeKey(userID, purpose)]
	if !ok {
		return repository.VerificationCode{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return row, nil
}

func (f *fakeVerificationCodeRepository) IncrementAttempts(_ context.Context, userID string, purpose repository.VerificationPurpose) (repository.VerificationCode, error) {
	key := verificationCodeKey(userID, purpose)
	row, ok := f.rows[key]
	if !ok {
		return repository.VerificationCode{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	row.Attempts++
	f.rows[key] = row
	return row, nil
}

func (f *fakeVerificationCodeRepository) Delete(_ context.Context, userID string, purpose repository.VerificationPurpose) error {
	delete(f.rows, verificationCodeKey(userID, purpose))
	return nil
}

// verificationCodeTargetKey/*ByTarget methods mirror Upsert/Get/
// IncrementAttempts/Delete exactly, keyed by (purpose, target) instead of
// (userID, purpose) — kept on a separate targetRows map rather than
// reusing rows, so a test userID can never collide with a target string.
func verificationCodeTargetKey(purpose repository.VerificationPurpose, target string) string {
	return string(purpose) + "|" + target
}

func (f *fakeVerificationCodeRepository) UpsertForSignup(
	_ context.Context, purpose repository.VerificationPurpose, target, codeHash string, expiresAt time.Time,
) (repository.VerificationCode, error) {
	row := repository.VerificationCode{
		ID:        verificationCodeTargetKey(purpose, target),
		Purpose:   purpose,
		Target:    target,
		CodeHash:  codeHash,
		Attempts:  0,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	f.targetRows[verificationCodeTargetKey(purpose, target)] = row
	return row, nil
}

func (f *fakeVerificationCodeRepository) GetByTarget(_ context.Context, purpose repository.VerificationPurpose, target string) (repository.VerificationCode, error) {
	row, ok := f.targetRows[verificationCodeTargetKey(purpose, target)]
	if !ok {
		return repository.VerificationCode{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	return row, nil
}

func (f *fakeVerificationCodeRepository) IncrementAttemptsByTarget(_ context.Context, purpose repository.VerificationPurpose, target string) (repository.VerificationCode, error) {
	key := verificationCodeTargetKey(purpose, target)
	row, ok := f.targetRows[key]
	if !ok {
		return repository.VerificationCode{}, fmt.Errorf("fake: %w", apperror.ErrNotFound)
	}
	row.Attempts++
	f.targetRows[key] = row
	return row, nil
}

func (f *fakeVerificationCodeRepository) DeleteByTarget(_ context.Context, purpose repository.VerificationPurpose, target string) error {
	delete(f.targetRows, verificationCodeTargetKey(purpose, target))
	return nil
}

// fakeEmailSender/fakeSmsSender capture what would have been sent, so tests
// can assert the logged/dispatched code matches what verifies successfully
// — same rigor the addendum's integration tests ask for, applied at the
// unit level too.
type fakeEmailSender struct {
	sent []struct {
		to, code string
		purpose  email.Purpose
	}
	err error
}

func (f *fakeEmailSender) SendVerificationCode(_ context.Context, to, code string, purpose email.Purpose) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, struct {
		to, code string
		purpose  email.Purpose
	}{to, code, purpose})
	return nil
}

func (f *fakeEmailSender) lastCode() string {
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1].code
}

type fakeSmsSender struct {
	sent []struct{ to, code string }
	err  error
}

func (f *fakeSmsSender) SendVerificationCode(_ context.Context, to, code string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, struct{ to, code string }{to, code})
	return nil
}

func (f *fakeSmsSender) lastCode() string {
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1].code
}

type publishedEvent struct {
	userID     string
	trustLevel int
}

type fakePublisher struct {
	published []publishedEvent
	err       error
}

func (f *fakePublisher) PublishUserOnboarded(_ context.Context, userID string, trustLevel int) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, publishedEvent{userID: userID, trustLevel: trustLevel})
	return nil
}

func (f *fakePublisher) Close() error { return nil }
