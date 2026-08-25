package repo

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/user"
)

// memRoleCatalog is the fixed role catalog the in-memory store validates grants
// against. It mirrors the rows seeded by migration 0006 (roles table).
var memRoleCatalog = map[string]bool{"user": true, "admin": true}

// Memory is a hermetic in-memory Store for unit tests and local dev.
type Memory struct {
	mu            sync.Mutex
	byID          map[string]model.Identity
	byEmail       map[string]string // email -> id
	byUserKey     map[string]string // public user key -> id
	handleOwners  map[string]string // current and retired handles -> id
	oidcSubjects  map[string]string // identity id + sector -> subject
	subjectOwners map[string]string // OIDC subject -> identity id
	pwHash        map[string]string // id -> hash
	profiles      map[string]model.Profile
	sessions      map[string]model.Session
	guestSessions map[string]model.GuestSession // sha256 token hash -> session
	failCount     map[string]int
	lockUntil     map[string]time.Time
	now           func() time.Time
	clients       map[string]model.OIDCClient
	keys          []model.SigningKey
	oauthLinks    map[string]string          // provider+"\x00"+providerUID -> identity id
	verifs        map[string]memVerification // token_hash -> verification record
	roles         map[string]map[string]bool // identity id -> set of granted role slugs
	audit         []AuditRow                 // append-only audit log (newest is highest index)
	auditSeq      int64                      // monotonically incrementing ID counter
	pats          map[int64]PATRow           // id -> PAT row
	patSeq        int64                      // monotonically incrementing PAT ID counter
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
		byUserKey: map[string]string{}, handleOwners: map[string]string{},
		oidcSubjects: map[string]string{}, subjectOwners: map[string]string{},
		pwHash: map[string]string{}, profiles: map[string]model.Profile{},
		sessions: map[string]model.Session{}, guestSessions: map[string]model.GuestSession{},
		failCount: map[string]int{}, lockUntil: map[string]time.Time{},
		now:     time.Now,
		clients: map[string]model.OIDCClient{}, keys: nil,
		oauthLinks: map[string]string{},
		verifs:     map[string]memVerification{},
		roles:      map[string]map[string]bool{},
		pats:       map[int64]PATRow{},
	}
}

// oauthKey builds the composite map key for an (provider, providerUID) pair.
func oauthKey(provider, providerUID string) string {
	return provider + "\x00" + providerUID
}

func (m *Memory) CreateGuestSession(_ context.Context, session model.GuestSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.guestSessions[session.TokenHash] = session
	return nil
}

func (m *Memory) GetGuestSession(_ context.Context, tokenHash string) (model.GuestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.guestSessions[tokenHash]
	if !ok {
		return model.GuestSession{}, ErrGuestSessionMissing
	}
	return session, nil
}

func (m *Memory) ClaimGuestSession(_ context.Context, tokenHash, identityID string, claimedAt time.Time) (model.GuestSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.guestSessions[tokenHash]
	if !ok {
		return model.GuestSession{}, ErrGuestSessionMissing
	}
	if session.ClaimedIdentityID != "" && session.ClaimedIdentityID != identityID {
		return model.GuestSession{}, ErrGuestClaimConflict
	}
	if session.ClaimedIdentityID == "" {
		session.ClaimedIdentityID = identityID
		session.ClaimedAt = &claimedAt
		m.guestSessions[tokenHash] = session
	}
	return session, nil
}

func (m *Memory) CreateIdentityWithProfile(ctx context.Context, in NewIdentityInput) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, role := range in.Roles {
		if !memRoleCatalog[role] {
			return model.Identity{}, UnknownRoleError{Slug: role}
		}
	}
	if _, ok := m.byEmail[in.Email]; ok {
		return model.Identity{}, ErrEmailTaken
	}
	newID := in.ID
	if newID == "" {
		generated, err := identifier.New()
		if err != nil {
			return model.Identity{}, err
		}
		newID = generated.String()
	}
	userKey, err := m.allocatePublicKey(ctx, in.UserKey)
	if err != nil {
		return model.Identity{}, err
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" {
		locale = "zh-CN"
	}
	id := model.Identity{
		ID: newID, UserKey: userKey, Email: in.Email, Status: model.StatusActive,
		CreatedAt: m.now(), UpdatedAt: m.now(),
	}
	m.byID[id.ID] = id
	m.byEmail[in.Email] = id.ID
	m.byUserKey[userKey] = id.ID
	m.oidcSubjects[oidcSubjectKey(id.ID, "public")] = userKey
	m.subjectOwners[userKey] = id.ID
	m.pwHash[id.ID] = in.PasswordHash
	m.profiles[id.ID] = model.Profile{IdentityID: id.ID, DisplayName: in.DisplayName, Locale: locale}
	m.roles[id.ID] = map[string]bool{}
	for _, role := range in.Roles {
		m.roles[id.ID][role] = true
	}
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

func (m *Memory) GetByUserKey(_ context.Context, userKey string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byUserKey[userKey]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return m.byID[id], nil
}

func (m *Memory) GetUserKeysByIDs(_ context.Context, identityIDs []string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(identityIDs))
	for _, identityID := range identityIDs {
		if identity, ok := m.byID[identityID]; ok {
			out[identityID] = identity.UserKey
		}
	}
	return out, nil
}

func oidcSubjectKey(identityID, sector string) string { return identityID + "\x00" + sector }

func (m *Memory) GetByOIDCSubject(_ context.Context, subject string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.subjectOwners[subject]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return m.byID[id], nil
}

func (m *Memory) ResolveOIDCSubject(_ context.Context, identityID, subjectType, sector string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	identity, ok := m.byID[identityID]
	if !ok {
		return "", ErrIdentityMissing
	}
	if subjectType == "" || subjectType == "public" {
		return identity.UserKey, nil
	}
	if subjectType != "pairwise" || strings.TrimSpace(sector) == "" {
		return "", errors.New("invalid OIDC subject policy")
	}
	sectorKey := "pairwise:" + strings.TrimSpace(sector)
	key := oidcSubjectKey(identityID, sectorKey)
	if existing := m.oidcSubjects[key]; existing != "" {
		return existing, nil
	}
	for range 3 {
		subject, err := user.NewPairwiseSubject()
		if err != nil {
			return "", err
		}
		if _, collision := m.subjectOwners[subject]; collision {
			continue
		}
		m.oidcSubjects[key] = subject
		m.subjectOwners[subject] = identityID
		return subject, nil
	}
	return "", errors.New("pairwise subject collision")
}

func (m *Memory) ListOIDCSubjects(_ context.Context, identityID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[identityID]; !ok {
		return nil, ErrIdentityMissing
	}
	prefix := identityID + "\x00"
	out := make([]string, 0)
	for key, subject := range m.oidcSubjects {
		if strings.HasPrefix(key, prefix) {
			out = append(out, subject)
		}
	}
	sort.Strings(out)
	return out, nil
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
func (m *Memory) CreateOAuthIdentity(ctx context.Context, in NewOAuthIdentityInput) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, role := range in.Roles {
		if !memRoleCatalog[role] {
			return model.Identity{}, UnknownRoleError{Slug: role}
		}
	}
	if _, ok := m.oauthLinks[oauthKey(in.Provider, in.ProviderUID)]; ok {
		return model.Identity{}, ErrProviderUIDTaken
	}
	if in.Email != "" {
		if _, ok := m.byEmail[in.Email]; ok {
			return model.Identity{}, ErrEmailTaken
		}
	}
	userKey, err := m.allocatePublicKey(ctx, in.UserKey)
	if err != nil {
		return model.Identity{}, err
	}
	locale := strings.TrimSpace(in.Locale)
	if locale == "" {
		locale = "zh-CN"
	}
	identityID, err := identifier.New()
	if err != nil {
		return model.Identity{}, err
	}
	id := model.Identity{
		ID: identityID.String(), UserKey: userKey, Email: in.Email, EmailVerified: in.EmailVerified,
		Status: model.StatusActive, CreatedAt: m.now(), UpdatedAt: m.now(),
	}
	m.byID[id.ID] = id
	m.byUserKey[userKey] = id.ID
	m.oidcSubjects[oidcSubjectKey(id.ID, "public")] = userKey
	m.subjectOwners[userKey] = id.ID
	if in.Email != "" {
		m.byEmail[in.Email] = id.ID
	}
	m.profiles[id.ID] = model.Profile{IdentityID: id.ID, DisplayName: in.DisplayName, Locale: locale}
	m.oauthLinks[oauthKey(in.Provider, in.ProviderUID)] = id.ID
	m.roles[id.ID] = map[string]bool{}
	for _, role := range in.Roles {
		m.roles[id.ID][role] = true
	}
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

// ListOAuthCredentials returns the identity's bound oauth providers (the memory
// store does not retain per-link email, so Email is left blank).
func (m *Memory) ListOAuthCredentials(_ context.Context, identityID string) ([]OAuthCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []OAuthCredential{}
	for key, id := range m.oauthLinks {
		if id != identityID {
			continue
		}
		provider, _, _ := strings.Cut(key, "\x00")
		out = append(out, OAuthCredential{Provider: provider})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

// DeleteOAuthCredential removes the (identityID, provider) link; bool = deleted.
func (m *Memory) DeleteOAuthCredential(_ context.Context, identityID, provider string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, id := range m.oauthLinks {
		if id != identityID {
			continue
		}
		if p, _, _ := strings.Cut(key, "\x00"); p == provider {
			delete(m.oauthLinks, key)
			return true, nil
		}
	}
	return false, nil
}

func (m *Memory) CountActivePasskeys(context.Context, string) (int, error) {
	return 0, nil
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

// SetPasswordHash inserts-or-replaces the identity's password hash (no existing
// credential required) — used when an OAuth-only account sets an initial password.
func (m *Memory) SetPasswordHash(_ context.Context, identityID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	s = withSessionExpiry(s, 0)
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

func (m *Memory) UpdateSessionAuthentication(_ context.Context, s model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[s.ID]; !ok {
		return ErrSessionNotFound
	}
	s.Authentication = authentication.NormalizeLegacy(s.Authentication, s.CreatedAt)
	m.sessions[s.ID] = s
	return nil
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

func (m *Memory) RetryAfter(_ context.Context, key string) (time.Duration, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok := m.lockUntil[key]
	if !ok {
		return 0, false, nil
	}
	remaining := until.Sub(m.now())
	if remaining <= 0 {
		delete(m.lockUntil, key)
		return 0, false, nil
	}
	return remaining, true, nil
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

// UpdateProfile replaces the editable display fields of an identity's profile.
func (m *Memory) UpdateProfile(_ context.Context, identityID string, in ProfileUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[identityID]
	if !ok {
		return ErrIdentityMissing
	}
	if in.Handle != p.Handle && in.Handle != "" {
		if owner, claimed := m.handleOwners[in.Handle]; claimed && owner != identityID {
			return ErrHandleUnavailable
		}
	}
	if in.Handle != "" {
		m.handleOwners[in.Handle] = identityID
	}
	p.DisplayName = in.DisplayName
	p.Handle = in.Handle
	p.Bio = in.Bio
	p.Locale = in.Locale
	m.profiles[identityID] = p
	return nil
}

func (m *Memory) SetProfileSocialLinks(_ context.Context, identityID string, links []model.SocialLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, ok := m.profiles[identityID]
	if !ok {
		return ErrIdentityMissing
	}
	profile.SocialLinks = append([]model.SocialLink(nil), links...)
	m.profiles[identityID] = profile
	return nil
}

func (m *Memory) GetPublicUserByKey(_ context.Context, userKey string) (model.PublicUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byUserKey[userKey]
	if !ok || m.byID[id].Status != model.StatusActive {
		return model.PublicUser{}, ErrIdentityMissing
	}
	return publicUserFromMemory(m.byID[id], m.profiles[id]), nil
}

func (m *Memory) GetPublicUserByHandle(_ context.Context, handle string) (model.PublicUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.handleOwners[handle]
	if !ok || m.byID[id].Status != model.StatusActive {
		return model.PublicUser{}, ErrIdentityMissing
	}
	return publicUserFromMemory(m.byID[id], m.profiles[id]), nil
}

func (m *Memory) GetPublicUsersByKeys(_ context.Context, userKeys []string) ([]model.PublicUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.PublicUser, 0, len(userKeys))
	for _, userKey := range userKeys {
		id, ok := m.byUserKey[userKey]
		if !ok || m.byID[id].Status != model.StatusActive {
			continue
		}
		out = append(out, publicUserFromMemory(m.byID[id], m.profiles[id]))
	}
	return out, nil
}

func publicUserFromMemory(identity model.Identity, profile model.Profile) model.PublicUser {
	links := make([]model.SocialLink, len(profile.SocialLinks))
	copy(links, profile.SocialLinks)
	return model.PublicUser{
		UserKey: identity.UserKey, Handle: profile.Handle, DisplayName: profile.DisplayName,
		AvatarMediaKey: profile.AvatarMediaKey, CoverMediaKey: profile.CoverMediaKey,
		Bio: profile.Bio, SocialLinks: links,
	}
}

func (m *Memory) allocatePublicKey(ctx context.Context, input string) (string, error) {
	if strings.TrimSpace(input) != "" {
		publicKey, err := user.ParsePublicKey(input)
		if err != nil {
			return "", err
		}
		if _, exists := m.byUserKey[string(publicKey)]; exists {
			return "", errors.New("public user key collision")
		}
		return string(publicKey), nil
	}
	allocated, err := identifier.Allocate(ctx, identifier.CompactURLV1,
		func(_ context.Context, candidate identifier.Key) (identifier.ClaimResult, error) {
			if _, exists := m.byUserKey[candidate.String()]; exists {
				return identifier.Collision, nil
			}
			return identifier.Claimed, nil
		})
	return allocated.String(), err
}

// SetProfileImage updates one image's media-key + asset-id (kind "avatar" | "cover").
func (m *Memory) SetProfileImage(_ context.Context, identityID, kind, mediaKey, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.profiles[identityID]
	if !ok {
		return ErrIdentityMissing
	}
	if kind == "cover" {
		p.CoverMediaKey = mediaKey
		p.CoverAssetID = assetID
	} else {
		p.AvatarMediaKey = mediaKey
		p.AvatarAssetID = assetID
	}
	m.profiles[identityID] = p
	return nil
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
	// Defensively shallow-copy Detail: the value-copy of row still shares the
	// underlying map with the caller, so without this a caller mutating its map
	// after InsertAudit would alter the stored row (and vice-versa).
	if row.Detail != nil {
		detail := make(map[string]any, len(row.Detail))
		for k, v := range row.Detail {
			detail[k] = v
		}
		row.Detail = detail
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
	if offset < 0 {
		offset = 0
	}
	if offset >= len(matches) {
		return []AuditRow{}, nil
	}
	matches = matches[offset:]
	if len(matches) > limit {
		matches = matches[:limit]
	}

	// Return a copy to avoid leaking internal slice storage. Normalize Detail to a
	// non-nil empty map so this impl matches the dao.PG.QueryAudit contract
	// (QueryAudit always returns a non-nil Detail).
	out := make([]AuditRow, len(matches))
	copy(out, matches)
	for i := range out {
		if out[i].Detail == nil {
			out[i].Detail = map[string]any{}
		}
	}
	return out, nil
}

// copyScopes returns a fresh copy of a Scopes slice so that callers cannot
// alias internal state. A nil input produces a non-nil empty slice.
func copyScopes(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// InsertPAT creates a new PAT row, assigns a monotonically incrementing ID,
// auto-fills CreatedAt when zero, and defensively copies the Scopes slice.
func (m *Memory) InsertPAT(_ context.Context, row PATRow) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patSeq++
	row.ID = m.patSeq
	if row.CreatedAt.IsZero() {
		row.CreatedAt = m.now()
	}
	row.Scopes = copyScopes(row.Scopes)
	m.pats[row.ID] = row
	return row.ID, nil
}

// ListPATByIdentity returns all PATs owned by identityID, ordered newest-first
// (by ID DESC). Each returned row's Scopes slice is a copy.
func (m *Memory) ListPATByIdentity(_ context.Context, identityID string) ([]PATRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []PATRow
	for _, r := range m.pats {
		if r.IdentityID == identityID {
			r.Scopes = copyScopes(r.Scopes)
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// GetPATByHash looks up a PAT by its token hash. Returns (copy, true) when
// found or (zero, false) when not found. The returned Scopes slice is a copy.
func (m *Memory) GetPATByHash(_ context.Context, hash string) (PATRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.pats {
		if r.TokenHash == hash {
			r.Scopes = copyScopes(r.Scopes)
			return r, true, nil
		}
	}
	return PATRow{}, false, nil
}

// DeletePAT deletes the PAT only when it exists AND belongs to identityID.
// Returns (true, nil) when deleted, (false, nil) otherwise.
func (m *Memory) DeletePAT(_ context.Context, id int64, identityID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.pats[id]
	if !ok || r.IdentityID != identityID {
		return false, nil
	}
	delete(m.pats, id)
	return true, nil
}

// TouchPATLastUsed sets LastUsedAt on the PAT. It is a no-op (returns nil)
// when the id does not exist.
func (m *Memory) TouchPATLastUsed(_ context.Context, id int64, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.pats[id]
	if !ok {
		return nil
	}
	ts := t // copy so the stored pointer is not aliased to the caller's variable
	r.LastUsedAt = &ts
	m.pats[id] = r
	return nil
}

// CountPATByIdentity returns the number of PATs owned by identityID.
func (m *Memory) CountPATByIdentity(_ context.Context, identityID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, r := range m.pats {
		if r.IdentityID == identityID {
			n++
		}
	}
	return n, nil
}

// Compile-time guarantees.
var _ Store = (*Memory)(nil)
var _ ClientRepo = (*Memory)(nil)
var _ SigningKeyRepo = (*Memory)(nil)
