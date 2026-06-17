// Package repo declares identity-service storage interfaces (the seam between
// logic and concrete PG/Redis implementations) plus shared sentinel errors.
package repo

import (
	"context"
	"errors"
	"time"

	"platform/services/identity/internal/model"
)

var (
	ErrEmailTaken      = errors.New("email taken")
	ErrIdentityMissing = errors.New("identity not found")
	ErrSessionNotFound = errors.New("session not found")
)

// NewIdentityInput is an atomic identity+profile+password-credential creation.
type NewIdentityInput struct {
	Email        string // canonical
	DisplayName  string
	Locale       string
	PasswordHash string
}

type IdentityRepo interface {
	CreateIdentityWithProfile(ctx context.Context, in NewIdentityInput) (model.Identity, error)
	GetByEmail(ctx context.Context, email string) (model.Identity, error) // ErrIdentityMissing
	GetByID(ctx context.Context, id string) (model.Identity, error)       // ErrIdentityMissing
	GetPasswordHash(ctx context.Context, identityID string) (string, error)
}

type SessionStore interface {
	CreateSession(ctx context.Context, s model.Session, ttl time.Duration) error
	GetSession(ctx context.Context, id string) (model.Session, error) // ErrSessionNotFound
	DeleteSession(ctx context.Context, id string) error
	ListSessionsByIdentity(ctx context.Context, identityID string) ([]model.Session, error)
	DeleteSessionsByIdentity(ctx context.Context, identityID string) error
}

// LoginThrottle tracks failed-login counters for rate-limit / lockout.
type LoginThrottle interface {
	Locked(ctx context.Context, key string) (bool, error)
	RecordFailure(ctx context.Context, key string, window, lockDur time.Duration, max int) error
	Reset(ctx context.Context, key string) error
}

// Store is the full repository surface (a single impl satisfies all three).
type Store interface {
	IdentityRepo
	SessionStore
	LoginThrottle
}
