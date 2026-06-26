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
	ErrEmailTaken          = errors.New("email taken")
	ErrIdentityMissing     = errors.New("identity not found")
	ErrSessionNotFound     = errors.New("session not found")
	ErrClientNotFound      = errors.New("oidc client not found")
	ErrNoActiveKey         = errors.New("no active signing key")
	ErrProviderUIDTaken    = errors.New("provider uid already linked")
	ErrVerificationInvalid = errors.New("verification token invalid, expired, or used")
	ErrUnknownRole         = errors.New("unknown role slug")
)

// Verification purpose scopes (a token issued for one purpose must not work for
// another — 登录码 ≠ 找回码).
const (
	PurposeVerifyEmail   = "verify_email"
	PurposePasswordReset = "password_reset"
)

// NewIdentityInput is an atomic identity+profile+password-credential creation.
type NewIdentityInput struct {
	Email        string // canonical
	DisplayName  string
	Locale       string
	PasswordHash string
}

// ProfileUpdate carries the user-editable display fields for a profile.
type ProfileUpdate struct {
	DisplayName string
	Username    string
	AvatarURL   string
	CoverURL    string
	Bio         string
	SocialLinks []model.SocialLink
	Locale      string
}

// AdminUserFilter constrains AdminListUsers. All fields are optional (zero value
// = no constraint). Keyword matches email / display_name / username (ILIKE).
type AdminUserFilter struct {
	Keyword string
	Status  string // "" = any; else active|disabled|deleted
	Role    string // "" = any role; else a role slug the identity must hold
	OrderBy string // "created_at" (default) | "display_name"
	Order   string // "asc" | "desc" (default desc)
	Limit   int    // 0 → default 20; capped at 100
	Offset  int
}

// AdminUserRow is one identity as surfaced to the admin user-management list:
// the identity row joined with its profile display fields and its role slugs.
type AdminUserRow struct {
	ID            string
	Email         string
	EmailVerified bool
	Status        model.Status
	CreatedAt     time.Time
	DisplayName   string
	Username      string
	AvatarURL     string
	Roles         []string
}

type IdentityRepo interface {
	CreateIdentityWithProfile(ctx context.Context, in NewIdentityInput) (model.Identity, error)
	GetByEmail(ctx context.Context, email string) (model.Identity, error)            // ErrIdentityMissing
	GetByID(ctx context.Context, id string) (model.Identity, error)                  // ErrIdentityMissing
	GetPasswordHash(ctx context.Context, identityID string) (string, error)
	GetProfile(ctx context.Context, identityID string) (model.Profile, error)        // ErrIdentityMissing
	// UpdateProfile replaces the user-editable display fields of an identity's
	// profile. Returns ErrIdentityMissing if the profile row does not exist.
	UpdateProfile(ctx context.Context, identityID string, in ProfileUpdate) error
	// SetProfileImage updates one image's url + asset-id columns (kind "avatar" |
	// "cover") without touching the other editable fields. ErrIdentityMissing if absent.
	SetProfileImage(ctx context.Context, identityID, kind, url, assetID string) error
	// SetEmailVerified flips the identity's email_verified flag.
	SetEmailVerified(ctx context.Context, identityID string, verified bool) error
	// UpdatePasswordHash replaces the identity's stored bcrypt password hash.
	// Returns ErrIdentityMissing if the identity has no password credential yet.
	UpdatePasswordHash(ctx context.Context, identityID, passwordHash string) error
	// SetPasswordHash inserts-or-replaces the identity's password hash. Unlike
	// UpdatePasswordHash it does NOT require an existing credential — it's how an
	// OAuth-only account adds an initial password.
	SetPasswordHash(ctx context.Context, identityID, passwordHash string) error
	// AdminListUsers returns a page of identities (joined with profile + roles)
	// matching the filter, plus the total count for that filter (ignoring paging).
	AdminListUsers(ctx context.Context, f AdminUserFilter) ([]AdminUserRow, int, error)
	// AdminUserStatusCounts returns the identity count per status value
	// (keys: "active", "disabled", "deleted") plus "total" (active+disabled,
	// excluding soft-deleted) for the admin dashboard.
	AdminUserStatusCounts(ctx context.Context) (map[string]int, error)
	// SetIdentityStatus updates an identity's lifecycle status. Returns
	// ErrIdentityMissing if no row matched.
	SetIdentityStatus(ctx context.Context, identityID string, status model.Status) error
}

// NewVerificationInput records an issued email token (stored hashed, with TTL).
type NewVerificationInput struct {
	IdentityID string
	Email      string
	Purpose    string // PurposeVerifyEmail | PurposePasswordReset
	TokenHash  string // sha256 hex
	ExpiresAt  time.Time
}

// VerificationRecord is the subset of a consumed token's row the logic needs.
type VerificationRecord struct {
	IdentityID string
	Email      string
}

// VerificationRepo persists and atomically consumes single-use email tokens.
type VerificationRepo interface {
	CreateVerification(ctx context.Context, in NewVerificationInput) error
	// ConsumeVerification atomically finds an unused, unexpired token matching
	// (tokenHash, purpose), marks it used, and returns its identity/email.
	// Returns ErrVerificationInvalid if none matches.
	ConsumeVerification(ctx context.Context, tokenHash, purpose string) (VerificationRecord, error)
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

// OAuthCredential is one external-provider credential bound to an identity.
type OAuthCredential struct {
	Provider string
	Email    string // may be "" (the in-memory store does not retain it)
}

// OAuthRepo resolves and persists external-provider identity links.
type OAuthRepo interface {
	// GetByProviderUID returns the linked identity, or ErrIdentityMissing if none.
	GetByProviderUID(ctx context.Context, provider, providerUID string) (model.Identity, error)
	// CreateOAuthIdentity atomically creates identity+profile+oauth credential.
	CreateOAuthIdentity(ctx context.Context, in NewOAuthIdentityInput) (model.Identity, error)
	// LinkOAuthCredential attaches an oauth credential to an existing identity.
	LinkOAuthCredential(ctx context.Context, identityID, provider, providerUID, email string, emailVerified bool) error
	// ListOAuthCredentials returns the identity's bound oauth credentials (sorted by provider).
	ListOAuthCredentials(ctx context.Context, identityID string) ([]OAuthCredential, error)
	// DeleteOAuthCredential removes the (identityID, provider) credential; bool = a row was deleted.
	DeleteOAuthCredential(ctx context.Context, identityID, provider string) (bool, error)
}

// RoleRepo manages the coarse-grained RBAC role grants for an identity. The
// catalog is the small fixed set seeded by migration 0006 (user, admin).
type RoleRepo interface {
	// GetRoles returns the identity's role slugs (sorted, possibly empty).
	GetRoles(ctx context.Context, identityID string) ([]string, error)
	// GrantRole is idempotent; returns ErrUnknownRole if slug is not in the catalog.
	GrantRole(ctx context.Context, identityID, slug string) error
	RevokeRole(ctx context.Context, identityID, slug string) error
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

// AuditRow is a single audit-log entry (maps 1-to-1 with the audit_logs table).
// Empty string fields correspond to NULL in Postgres.
type AuditRow struct {
	ID         int64
	Event      string
	ActorID    string         // "" → NULL in PG
	TargetID   string         // "" → NULL in PG
	ActorEmail string
	IP         string
	UserAgent  string
	ClientID   string
	RequestID  string
	Result     string         // "success" | "failure"
	Detail     map[string]any
	OccurredAt time.Time
}

// AuditFilter constrains a QueryAudit call. IdentityID matches actor OR target;
// all fields are optional (zero value = no constraint).
type AuditFilter struct {
	IdentityID string
	Event      string
	Limit      int // 0 → default 50; capped at 200
	Offset     int
}

// AuditRepo persists and queries structured audit events.
type AuditRepo interface {
	InsertAudit(ctx context.Context, row AuditRow) error
	// QueryAudit returns rows newest-first (by id desc).
	QueryAudit(ctx context.Context, f AuditFilter) ([]AuditRow, error)
}

// PATRow is a single personal-access-token record (maps 1-to-1 with pat_tokens).
type PATRow struct {
	ID          int64
	IdentityID  string
	Name        string
	TokenHash   string
	TokenPrefix string
	Scopes      []string
	ExpiresAt   *time.Time // nil = never expires
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// PATRepo persists and queries personal access tokens.
type PATRepo interface {
	// InsertPAT creates a new PAT and returns its assigned id.
	InsertPAT(ctx context.Context, row PATRow) (int64, error)
	// ListPATByIdentity returns all PATs for an identity, newest-first (id DESC).
	ListPATByIdentity(ctx context.Context, identityID string) ([]PATRow, error)
	// GetPATByHash looks up a PAT by its token hash for verification; bool=found.
	GetPATByHash(ctx context.Context, hash string) (PATRow, bool, error)
	// DeletePAT deletes the token only if it exists AND belongs to identityID.
	// Returns whether a row was actually deleted.
	DeletePAT(ctx context.Context, id int64, identityID string) (bool, error)
	// TouchPATLastUsed sets the LastUsedAt timestamp on the token.
	TouchPATLastUsed(ctx context.Context, id int64, t time.Time) error
	// CountPATByIdentity returns the number of PATs for an identity.
	CountPATByIdentity(ctx context.Context, identityID string) (int, error)
}

// Store is the full repository surface (a single impl satisfies all three).
type Store interface {
	IdentityRepo
	SessionStore
	LoginThrottle
	OAuthRepo
	VerificationRepo
	RoleRepo
	AuditRepo
	PATRepo
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
