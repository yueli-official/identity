// Package githubbinding verifies and persists GitHub account ownership as an
// identity fact. It is deliberately independent from OAuth login credentials:
// a binding authorizes provider-backed publishing flows, never account login.
package githubbinding

import (
	"context"
	"errors"
	"time"
)

const (
	ProviderGitHub = "github"

	StatusActive  = "active"
	StatusUnbound = "unbound"
	StatusBlocked = "blocked"
)

var (
	ErrUnavailable       = errors.New("github binding is unavailable")
	ErrInvalidAttempt    = errors.New("github binding attempt is invalid, expired, or consumed")
	ErrProviderFailure   = errors.New("github provider verification failed")
	ErrBindingConflict   = errors.New("github account is bound to another identity")
	ErrBindingNotFound   = errors.New("github binding not found")
	ErrBindingInactive   = errors.New("github binding is not active")
	ErrInvalidSubmission = errors.New("github submission manifest is invalid")
	ErrSubjectMismatch   = errors.New("github binding and publisher attestation subjects differ")
)

// Account is the trusted projection returned by GitHub's authenticated GET
// /user endpoint. AccountID is kept as a decimal string for cross-language
// stability; Login is display-only and may change.
type Account struct {
	AccountID string
	NodeID    string
	Login     string
	AvatarURL string
}

// Attempt is short-lived, one-time authorization state. State and session are
// stored only as SHA-256 digests. The PKCE verifier is encrypted at rest.
type Attempt struct {
	ID                 string
	StateDigest        string
	IdentityID         string
	SessionDigest      string
	VerifierCiphertext string
	ReturnTo           string
	ExpiresAt          time.Time
	ConsumedAt         *time.Time
	CreatedAt          time.Time
}

// Binding is append-only history. Unbind and provider revocation close the
// current row; a later verified authorization creates a new active row.
type Binding struct {
	ID                string
	IdentityID        string
	Provider          string
	ProviderAccountID string
	ProviderNodeID    string
	Login             string
	AvatarURL         string
	Status            string
	VerifiedAt        time.Time
	LastVerifiedAt    time.Time
	UnboundAt         *time.Time
	BlockedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type BindResult struct {
	Binding Binding
	Created bool
	Renamed bool
}

type Store interface {
	CreateAttempt(context.Context, Attempt) error
	ConsumeAttempt(context.Context, string, string, time.Time) (Attempt, error)
	Bind(context.Context, string, Account, time.Time) (BindResult, error)
	ListByIdentity(context.Context, string) ([]Binding, error)
	FindActiveByAccount(context.Context, string) (Binding, error)
	Unbind(context.Context, string, string, time.Time) (Binding, error)
	BlockByAccount(context.Context, string, string, time.Time) ([]Binding, error)
}

// Provider owns only GitHub App user authorization. Tokens are used to call
// authenticated GET /user, then revoked best-effort and never persisted.
type Provider interface {
	AuthorizationURL(state, codeChallenge string) string
	ExchangeCode(context.Context, string, string) (string, error)
	AuthenticatedUser(context.Context, string) (Account, error)
	RevokeAccessToken(context.Context, string) error
}

type Config struct {
	Store                   Store
	Provider                Provider
	CipherSecret            []byte
	AttemptTTL              time.Duration
	Now                     func() time.Time
	ResolvePublisherSubject func(context.Context, string) (string, error)
}

type BeginResult struct {
	AuthorizationURL string
	ExpiresAt        time.Time
}

type CompleteResult struct {
	Binding  Binding
	ReturnTo string
	Created  bool
	Renamed  bool
}
