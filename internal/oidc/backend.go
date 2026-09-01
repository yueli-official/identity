package oidc

import (
	"context"
	"errors"
	"time"
)

// Backend errors. Store (Task 5) maps these to fosite sentinels.
var (
	ErrBackendNotFound = errors.New("oidc backend: record not found")
	ErrBackendInactive = errors.New("oidc backend: record inactive") // rotated/consumed
)

// Generic transient-session kinds (access tokens are NOT stored: JWT is self-contained).
const (
	kindAuthCode = "authcode"
	kindPKCE     = "pkce"
	kindOIDC     = "oidc"
)

// Record is a generic transient session row (authcode/pkce/oidc).
type Record struct {
	RequestID string
	ClientID  string
	Subject   string
	Active    bool
	ExpiresAt time.Time
	Data      []byte // serialized fosite.Requester (storedRequest JSON)
}

// RefreshRecord is a refresh-token row with security columns.
type RefreshRecord struct {
	RequestID       string
	ClientID        string
	Subject         string
	SessionID       string
	AccessSignature string
	Active          bool
	ExpiresAt       time.Time
	Data            []byte
}

// Backend is the persistence seam beneath oidc.Store. memBackend (tests) and
// pgBackend (prod) implement it. Transactional methods are real on PG, no-op on
// memory.
type Backend interface {
	// generic transient sessions keyed by (kind, signature)
	PutGeneric(ctx context.Context, kind, signature string, r Record) error
	GetGeneric(ctx context.Context, kind, signature string) (Record, error) // ErrBackendNotFound
	DeactivateGeneric(ctx context.Context, kind, signature string) error
	DeleteGeneric(ctx context.Context, kind, signature string) error

	// refresh tokens
	PutRefresh(ctx context.Context, signature string, r RefreshRecord) error
	GetRefresh(ctx context.Context, signature string) (RefreshRecord, error) // ErrBackendNotFound / ErrBackendInactive
	DeactivateRefresh(ctx context.Context, signature string) error
	DeleteRefresh(ctx context.Context, signature string) error
	RevokeRefreshByRequestID(ctx context.Context, requestID string) error // family
	RevokeRefreshBySession(ctx context.Context, sessionID string) error   // passive logout (single session)
	RevokeRefreshBySubject(ctx context.Context, subject string) error     // logout-all
	PutRefreshReplay(ctx context.Context, receipt RefreshReplayReceipt) error
	GetRefreshReplay(ctx context.Context, keyDigest, clientID string, now time.Time) (RefreshReplayReceipt, bool, error)

	// transaction (no-op on memory)
	BeginTX(ctx context.Context) (context.Context, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
