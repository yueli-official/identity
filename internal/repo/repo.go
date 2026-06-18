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
	ErrEmailTaken       = errors.New("email taken")
	ErrIdentityMissing  = errors.New("identity not found")
	ErrSessionNotFound  = errors.New("session not found")
	ErrClientNotFound   = errors.New("oidc client not found")
	ErrNoActiveKey      = errors.New("no active signing key")
	ErrProviderUIDTaken = errors.New("provider uid already linked")
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
	GetByEmail(ctx context.Context, email string) (model.Identity, error)            // ErrIdentityMissing
	GetByID(ctx context.Context, id string) (model.Identity, error)                  // ErrIdentityMissing
	GetPasswordHash(ctx context.Context, identityID string) (string, error)
	GetProfile(ctx context.Context, identityID string) (model.Profile, error)        // ErrIdentityMissing
}

// NewOAuthIdentityInput atomically creates identity + profile + an OAuth credential
// (no password credential — OAuth-only accounts have no local password until they set one).
type NewOAuthIdentityInput struct {
	Email         string // canonical; may be "" only if the provider returned none
	EmailVerified bool
	DisplayName   string
	Locale        string
	Provider      string // e.g. "google"
	ProviderUID   string // provider's stable user id (Google "sub")
}

// OAuthRepo resolves and persists external-provider identity links.
type OAuthRepo interface {
	// GetByProviderUID returns the linked identity, or ErrIdentityMissing if none.
	GetByProviderUID(ctx context.Context, provider, providerUID string) (model.Identity, error)
	// CreateOAuthIdentity atomically creates identity+profile+oauth credential.
	CreateOAuthIdentity(ctx context.Context, in NewOAuthIdentityInput) (model.Identity, error)
	// LinkOAuthCredential attaches an oauth credential to an existing identity.
	LinkOAuthCredential(ctx context.Context, identityID, provider, providerUID, email string, emailVerified bool) error
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
	OAuthRepo
}

// ClientRepo provides read access to registered OIDC relying parties.
type ClientRepo interface {
	GetClient(ctx context.Context, id string) (model.OIDCClient, error) // ErrClientNotFound
}

// SigningKeyRepo manages RS256 key pairs used to sign OIDC tokens.
type SigningKeyRepo interface {
	GetActiveKey(ctx context.Context) (model.SigningKey, error) // ErrNoActiveKey
	InsertKey(ctx context.Context, k model.SigningKey) error
	ListPublicKeys(ctx context.Context) ([]model.SigningKey, error) // active + retired (for JWKS)
}
