package service

import (
	"context"
	"fmt"
	"time"

	"github.com/professional-connections/backend/services/auth/internal/repository"
	"github.com/professional-connections/backend/shared/apperror"
)

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

func (f *fakeUserRepository) Create(_ context.Context, u repository.NewUser) (repository.User, error) {
	if f.createErr != nil {
		return repository.User{}, f.createErr
	}
	f.createCalls = append(f.createCalls, u)
	f.nextID++
	created := repository.User{
		ID:              fmt.Sprintf("user-%d", f.nextID),
		LinkedInSub:     u.LinkedInSub,
		FullName:        u.FullName,
		ProfilePhotoURL: u.ProfilePhotoURL,
		TrustLevel:      u.TrustLevel,
		AccountStatus:   repository.AccountStatusActive,
	}
	f.byLinkedInSub[u.LinkedInSub] = created
	f.byID[created.ID] = created
	return created, nil
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
