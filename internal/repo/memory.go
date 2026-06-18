package repo

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"platform/services/identity/internal/model"
)

// memRoleCatalog is the fixed role catalog the in-memory store validates grants
// against. It mirrors the rows seeded by migration 0006 (roles table).
var memRoleCatalog = map[string]bool{"user": true, "admin": true}

// Memory is a hermetic in-memory Store for unit tests and local dev.
type Memory struct {
	mu        sync.Mutex
	byID      map[string]model.Identity
	byEmail   map[string]string // email -> id
	pwHash    map[string]string // id -> hash
	profiles  map[string]model.Profile
	sessions  map[string]model.Session
	failCount map[string]int
	lockUntil map[string]time.Time
	now        func() time.Time
	clients    map[string]model.OIDCClient
	keys       []model.SigningKey
	oauthLinks map[string]string // provider+"\x00"+providerUID -> identity id
	verifs     map[string]memVerification // token_hash -> verification record
	roles      map[string]map[string]bool // identity id -> set of granted role slugs
	audit      []AuditRow                 // append-only audit log (newest is highest index)
	auditSeq   int64                      // monotonically incrementing ID counter
}

// memVerification mirrors a single email_verifications row in memory.
type memVerification struct {
	IdentityID string
	Email      string
	Purpose    string
	ExpiresAt  time.Time
	Used       bool
}

func NewMemory() *Memory {
	return &Memory{
		byID: map[string]model.Identity{}, byEmail: map[string]string{},
		pwHash: map[string]string{}, profiles: map[string]model.Profile{},
		sessions: map[string]model.Session{}, failCount: map[string]int{},
		lockUntil: map[string]time.Time{}, now: time.Now,
		clients: map[string]model.OIDCClient{}, keys: nil,
		oauthLinks: map[string]string{},
		verifs:     map[string]memVerification{},
		roles:      map[string]map[string]bool{},
	}
}

// oauthKey builds the composite map key for an (provider, providerUID) pair.
func oauthKey(provider, providerUID string) string {
	return provider + "\x00" + providerUID
}

func (m *Memory) CreateIdentityWithProfile(_ context.Context, in NewIdentityInput) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[in.Email]; ok {
		return model.Identity{}, ErrEmailTaken
	}
	id := model.Identity{
		ID: uuid.NewString(), Email: in.Email, Status: model.StatusActive,
		CreatedAt: m.now(), UpdatedAt: m.now(),
	}
	m.byID[id.ID] = id
	m.byEmail[in.Email] = id.ID
	m.pwHash[id.ID] = in.PasswordHash
	m.profiles[id.ID] = model.Profile{IdentityID: id.ID, DisplayName: in.DisplayName, Locale: in.Locale}
	return id, nil
}

func (m *Memory) GetByEmail(_ context.Context, email string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byEmail[email]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return m.byID[id], nil
}

func (m *Memory) GetByID(_ context.Context, id string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.byID[id]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return v, nil
}

func (m *Memory) GetPasswordHash(_ context.Context, identityID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.pwHash[identityID]
	if !ok {
		return "", ErrIdentityMissing
	}
	return h, nil
}

// GetByProviderUID returns the identity linked to (provider, providerUID).
// Returns ErrIdentityMissing when no link exists.
func (m *Memory) GetByProviderUID(_ context.Context, provider, providerUID string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.oauthLinks[oauthKey(provider, providerUID)]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return m.byID[id], nil
}

// CreateOAuthIdentity atomically creates an identity + profile + oauth link
// (no password credential). Returns ErrEmailTaken on email collision and
// ErrProviderUIDTaken when the (provider, providerUID) key is already linked.
func (m *Memory) CreateOAuthIdentity(_ context.Context, in NewOAuthIdentityInput) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.oauthLinks[oauthKey(in.Provider, in.ProviderUID)]; ok {
		return model.Identity{}, ErrProviderUIDTaken
	}
	if in.Email != "" {
		if _, ok := m.byEmail[in.Email]; ok {
			return model.Identity{}, ErrEmailTaken
		}
	}
	id := model.Identity{
		ID: uuid.NewString(), Email: in.Email, EmailVerified: in.EmailVerified,
		Status: model.StatusActive, CreatedAt: m.now(), UpdatedAt: m.now(),
	}
	m.byID[id.ID] = id
	if in.Email != "" {
		m.byEmail[in.Email] = id.ID
	}
	m.profiles[id.ID] = model.Profile{IdentityID: id.ID, DisplayName: in.DisplayName, Locale: in.Locale}
	m.oauthLinks[oauthKey(in.Provider, in.ProviderUID)] = id.ID
	return id, nil
}

// LinkOAuthCredential records an oauth link against an existing identity.
// Returns ErrProviderUIDTaken when the (provider, providerUID) key already exists.
func (m *Memory) LinkOAuthCredential(_ context.Context, identityID, provider, providerUID, _ string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := oauthKey(provider, providerUID)
	if _, ok := m.oauthLinks[key]; ok {
		return ErrProviderUIDTaken
	}
	m.oauthLinks[key] = identityID
	return nil
}

// SetEmailVerified flips the stored identity's email_verified flag.
// Returns ErrIdentityMissing when the identity does not exist.
func (m *Memory) SetEmailVerified(_ context.Context, identityID string, verified bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byID[identityID]
	if !ok {
		return ErrIdentityMissing
	}
	id.EmailVerified = verified
	id.UpdatedAt = m.now()
	m.byID[identityID] = id
	return nil
}

// UpdatePasswordHash replaces the stored bcrypt password hash for an identity.
// Returns ErrIdentityMissing when the identity has no password credential.
func (m *Memory) UpdatePasswordHash(_ context.Context, identityID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pwHash[identityID]; !ok {
		return ErrIdentityMissing
	}
	m.pwHash[identityID] = passwordHash
	return nil
}

// CreateVerification records an issued email token keyed by its hash.
func (m *Memory) CreateVerification(_ context.Context, in NewVerificationInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifs[in.TokenHash] = memVerification{
		IdentityID: in.IdentityID, Email: in.Email,
		Purpose: in.Purpose, ExpiresAt: in.ExpiresAt,
	}
	return nil
}

// ConsumeVerification atomically validates and marks a token used (single-use).
// Returns ErrVerificationInvalid when no unused, unexpired token matches both
// the hash and the purpose.
func (m *Memory) ConsumeVerification(_ context.Context, tokenHash, purpose string) (VerificationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.verifs[tokenHash]
	if !ok || v.Used || v.Purpose != purpose || !v.ExpiresAt.After(m.now()) {
		return VerificationRecord{}, ErrVerificationInvalid
	}
	v.Used = true
	m.verifs[tokenHash] = v
	return VerificationRecord{IdentityID: v.IdentityID, Email: v.Email}, nil
}

// GetRoles returns the identity's granted role slugs, sorted (possibly empty).
func (m *Memory) GetRoles(_ context.Context, identityID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.roles[identityID]
	out := make([]string, 0, len(set))
	for slug := range set {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out, nil
}

// GrantRole grants a catalog role to the identity. It is idempotent and returns
// ErrUnknownRole when slug is not in the catalog.
func (m *Memory) GrantRole(_ context.Context, identityID, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !memRoleCatalog[slug] {
		return ErrUnknownRole
	}
	set := m.roles[identityID]
	if set == nil {
		set = map[string]bool{}
		m.roles[identityID] = set
	}
	set[slug] = true
	return nil
}

// RevokeRole removes a role grant from the identity. Revoking a role the
// identity does not have is a no-op.
func (m *Memory) RevokeRole(_ context.Context, identityID, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.roles[identityID], slug)
	return nil
}

func (m *Memory) CreateSession(_ context.Context, s model.Session, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *Memory) GetSession(_ context.Context, id string) (model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return model.Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (m *Memory) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Memory) ListSessionsByIdentity(_ context.Context, identityID string) ([]model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Session
	for _, s := range m.sessions {
		if s.IdentityID == identityID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *Memory) DeleteSessionsByIdentity(_ context.Context, identityID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if s.IdentityID == identityID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *Memory) Locked(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok := m.lockUntil[key]
	return ok && m.now().Before(until), nil
}

func (m *Memory) RecordFailure(_ context.Context, key string, _, lockDur time.Duration, max int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount[key]++
	if m.failCount[key] >= max {
		m.lockUntil[key] = m.now().Add(lockDur)
	}
	return nil
}

func (m *Memory) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failCount, key)
	delete(m.lockUntil, key)
	return nil
}

// GetProfile returns the stored profile for an identity.
// Returns ErrIdentityMissing when not found.
func (m *Memory) GetProfile(_ context.Context, identityID string) (model.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[identityID]
	if !ok {
		return model.Profile{}, ErrIdentityMissing
	}
	return p, nil
}

// SetClient is a test helper that registers an OIDCClient by its ID.
func (m *Memory) SetClient(c model.OIDCClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.ID] = c
}

// GetClient returns the OIDCClient with the given id.
// Returns ErrClientNotFound when absent.
func (m *Memory) GetClient(_ context.Context, id string) (model.OIDCClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[id]
	if !ok {
		return model.OIDCClient{}, ErrClientNotFound
	}
	return c, nil
}

// GetActiveKey returns the first key with Status == model.KeyActive.
// Returns ErrNoActiveKey when none exists.
func (m *Memory) GetActiveKey(_ context.Context) (model.SigningKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.keys {
		if k.Status == model.KeyActive {
			return k, nil
		}
	}
	return model.SigningKey{}, ErrNoActiveKey
}

// InsertKey appends a signing key to the in-memory store.
func (m *Memory) InsertKey(_ context.Context, k model.SigningKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, k)
	return nil
}

// ListPublicKeys returns all keys (active + retired) for JWKS exposure.
func (m *Memory) ListPublicKeys(_ context.Context) ([]model.SigningKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.SigningKey, len(m.keys))
	copy(out, m.keys)
	return out, nil
}

// InsertAudit appends an audit row to the in-memory log. It assigns a
// monotonically incrementing ID and sets OccurredAt to now() when zero.
func (m *Memory) InsertAudit(_ context.Context, row AuditRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditSeq++
	row.ID = m.auditSeq
	if row.OccurredAt.IsZero() {
		row.OccurredAt = m.now()
	}
	m.audit = append(m.audit, row)
	return nil
}

// QueryAudit returns audit rows matching f, ordered newest-first (by ID desc).
// IdentityID matches actor OR target; Event matches exactly; Limit 0 → 50,
// capped at 200; Offset skips the first N matching rows. Returns a copy slice
// so callers cannot mutate internal state.
func (m *Memory) QueryAudit(_ context.Context, f AuditFilter) ([]AuditRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect matching rows in reverse insertion order (newest-first).
	var matches []AuditRow
	for i := len(m.audit) - 1; i >= 0; i-- {
		r := m.audit[i]
		if f.IdentityID != "" && r.ActorID != f.IdentityID && r.TargetID != f.IdentityID {
			continue
		}
		if f.Event != "" && r.Event != f.Event {
			continue
		}
		matches = append(matches, r)
	}

	// Apply limit/offset.
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset >= len(matches) {
		return []AuditRow{}, nil
	}
	matches = matches[offset:]
	if len(matches) > limit {
		matches = matches[:limit]
	}

	// Return a copy to avoid leaking internal slice storage.
	out := make([]AuditRow, len(matches))
	copy(out, matches)
	return out, nil
}

// Compile-time guarantees.
var _ Store = (*Memory)(nil)
var _ ClientRepo = (*Memory)(nil)
var _ SigningKeyRepo = (*Memory)(nil)
