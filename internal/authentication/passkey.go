package authentication

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCeremonyInvalid      = errors.New("authentication ceremony invalid")
	ErrCeremonyExpired      = errors.New("authentication ceremony expired")
	ErrCeremonyConsumed     = errors.New("authentication ceremony consumed")
	ErrPasskeyExists        = errors.New("passkey credential already exists")
	ErrPasskeyNotFound      = errors.New("passkey credential not found")
	ErrPasskeyConcurrentUse = errors.New("passkey credential changed concurrently")
	ErrPasskeyUnavailable   = errors.New("passkey authentication unavailable")
	ErrLastAuthenticator    = errors.New("cannot remove the last login authenticator")
)

type CeremonyKind string

const (
	CeremonyPasskeyRegistration   CeremonyKind = "passkey_registration"
	CeremonyPasskeyAuthentication CeremonyKind = "passkey_authentication"
)

type PasskeyCredential struct {
	ID                           string
	IdentityID                   string
	RPID                         string
	CredentialID                 []byte
	PublicKey                    []byte
	PublicKeyAlgorithm           int64
	Transports                   []string
	Attachment                   string
	AttestationType              string
	AttestationFormat            string
	AAGUID                       []byte
	SignCount                    uint32
	CloneWarning                 bool
	Flags                        byte
	UserVerified                 bool
	UserVerifiedAtRegistration   bool
	BackupEligible               bool
	BackupState                  bool
	AttestationClientDataJSON    []byte
	AttestationClientDataHash    []byte
	AttestationAuthenticatorData []byte
	AttestationObject            []byte
	Status                       string
	Label                        string
	Version                      int64
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	LastUsedAt                   *time.Time
}

type PasskeyUser struct {
	IdentityID  string
	UserHandle  []byte
	Name        string
	DisplayName string
	Credentials []PasskeyCredential
}

type Ceremony struct {
	ID              string
	Kind            CeremonyKind
	IdentityID      string
	SessionID       string
	ChallengeDigest []byte
	LibraryState    []byte
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	FailedAttempts  int
	CreatedAt       time.Time
}

type PasskeyStore interface {
	GetOrCreatePasskeyUser(context.Context, string, []byte) ([]byte, error)
	GetPasskeyUserByIdentity(context.Context, string) (PasskeyUser, error)
	GetPasskeyUserByHandle(context.Context, []byte) (PasskeyUser, error)
	ListPasskeys(context.Context, string) ([]PasskeyCredential, error)
	RenamePasskey(context.Context, string, string, string, time.Time) (PasskeyCredential, error)
	RevokePasskey(context.Context, string, string, string, time.Time) error

	CreateCeremony(context.Context, Ceremony) error
	GetCeremony(context.Context, string) (Ceremony, error)
	RecordCeremonyFailure(context.Context, string, int) error
	CompletePasskeyRegistration(context.Context, Ceremony, PasskeyCredential) error
	CompletePasskeyAuthentication(context.Context, Ceremony, PasskeyCredential, Session, time.Duration) error
}

type SessionCache interface {
	CreateSession(context.Context, Session, time.Duration) error
}

type CeremonyMaterial struct {
	ChallengeDigest []byte
	LibraryState    []byte
}

type BrowserOptions struct {
	JSON json.RawMessage
}

type WebAuthnVerifier interface {
	BeginRegistration(PasskeyUser) (CeremonyMaterial, BrowserOptions, error)
	FinishRegistration(PasskeyUser, CeremonyMaterial, []byte) (PasskeyCredential, error)
	BeginDiscoverableAuthentication() (CeremonyMaterial, BrowserOptions, error)
	FinishDiscoverableAuthentication(
		CeremonyMaterial,
		[]byte,
		func([]byte) (PasskeyUser, error),
	) (PasskeyUser, PasskeyCredential, error)
}

type ModuleConfig struct {
	SessionTTL     time.Duration
	CeremonyTTL    time.Duration
	TransactionTTL time.Duration
	RecoveryTTL    time.Duration
}

type Module struct {
	store    PasskeyStore
	cache    SessionCache
	verifier WebAuthnVerifier
	events   SecurityEventSink
	mfa      MFAStore
	totp     TOTPVerifier
	secrets  *SecretBox
	recovery *RecoveryCodeCodec
	cfg      ModuleConfig
	now      func() time.Time
}

func NewModule(store PasskeyStore, cache SessionCache, verifier WebAuthnVerifier, cfg ModuleConfig) (*Module, error) {
	if store == nil || verifier == nil {
		return nil, ErrPasskeyUnavailable
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	if cfg.CeremonyTTL <= 0 {
		cfg.CeremonyTTL = 5 * time.Minute
	}
	if cfg.TransactionTTL <= 0 {
		cfg.TransactionTTL = 5 * time.Minute
	}
	if cfg.RecoveryTTL <= 0 {
		cfg.RecoveryTTL = 15 * time.Minute
	}
	return &Module{store: store, cache: cache, verifier: verifier, cfg: cfg, now: time.Now}, nil
}

type BeginPasskeyRegistrationRequest struct {
	IdentityID  string
	SessionID   string
	Name        string
	DisplayName string
}

type BeginCeremonyResult struct {
	CeremonyID string
	ExpiresAt  time.Time
	Options    json.RawMessage
}

func (module *Module) BeginPasskeyRegistration(
	ctx context.Context,
	request BeginPasskeyRegistrationRequest,
) (BeginCeremonyResult, error) {
	handleCandidate := make([]byte, 64)
	if _, err := rand.Read(handleCandidate); err != nil {
		return BeginCeremonyResult{}, err
	}
	handle, err := module.store.GetOrCreatePasskeyUser(ctx, request.IdentityID, handleCandidate)
	if err != nil {
		return BeginCeremonyResult{}, err
	}
	user, err := module.store.GetPasskeyUserByIdentity(ctx, request.IdentityID)
	if err != nil {
		return BeginCeremonyResult{}, err
	}
	user.UserHandle = handle
	user.Name = request.Name
	user.DisplayName = request.DisplayName
	material, options, err := module.verifier.BeginRegistration(user)
	if err != nil {
		return BeginCeremonyResult{}, ErrPasskeyUnavailable
	}
	now := module.now().UTC()
	ceremony := Ceremony{
		ID: uuid.NewString(), Kind: CeremonyPasskeyRegistration,
		IdentityID: request.IdentityID, SessionID: request.SessionID,
		ChallengeDigest: material.ChallengeDigest, LibraryState: material.LibraryState,
		ExpiresAt: now.Add(module.cfg.CeremonyTTL), CreatedAt: now,
	}
	if err := module.store.CreateCeremony(ctx, ceremony); err != nil {
		return BeginCeremonyResult{}, err
	}
	return BeginCeremonyResult{
		CeremonyID: ceremony.ID, ExpiresAt: ceremony.ExpiresAt, Options: options.JSON,
	}, nil
}

type FinishPasskeyRegistrationRequest struct {
	CeremonyID string
	SessionID  string
	Label      string
	Response   []byte
}

func (module *Module) FinishPasskeyRegistration(
	ctx context.Context,
	request FinishPasskeyRegistrationRequest,
) (PasskeyCredential, error) {
	ceremony, err := module.loadCeremony(ctx, request.CeremonyID, CeremonyPasskeyRegistration)
	if err != nil {
		return PasskeyCredential{}, err
	}
	if ceremony.SessionID == "" || ceremony.SessionID != request.SessionID {
		return PasskeyCredential{}, ErrCeremonyInvalid
	}
	user, err := module.store.GetPasskeyUserByIdentity(ctx, ceremony.IdentityID)
	if err != nil {
		return PasskeyCredential{}, ErrCeremonyInvalid
	}
	credential, err := module.verifier.FinishRegistration(user, CeremonyMaterial{
		ChallengeDigest: ceremony.ChallengeDigest, LibraryState: ceremony.LibraryState,
	}, request.Response)
	if err != nil {
		module.recordFailure(ctx, ceremony)
		return PasskeyCredential{}, ErrCeremonyInvalid
	}
	now := module.now().UTC()
	credential.ID = uuid.NewString()
	credential.IdentityID = ceremony.IdentityID
	credential.Label = request.Label
	credential.Status = "active"
	credential.Version = 1
	credential.CreatedAt = now
	credential.UpdatedAt = now
	if err := module.store.CompletePasskeyRegistration(ctx, ceremony, credential); err != nil {
		return PasskeyCredential{}, err
	}
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventPasskeyRegistered, IdentityID: credential.IdentityID,
		CredentialID: credential.ID, Label: credential.Label, OccurredAt: now,
		Detail: map[string]any{
			"attachment":     credential.Attachment,
			"backupEligible": credential.BackupEligible,
		},
	})
	return credential, nil
}

func (module *Module) BeginPasskeyAuthentication(ctx context.Context) (BeginCeremonyResult, error) {
	material, options, err := module.verifier.BeginDiscoverableAuthentication()
	if err != nil {
		return BeginCeremonyResult{}, ErrPasskeyUnavailable
	}
	now := module.now().UTC()
	ceremony := Ceremony{
		ID: uuid.NewString(), Kind: CeremonyPasskeyAuthentication,
		ChallengeDigest: material.ChallengeDigest, LibraryState: material.LibraryState,
		ExpiresAt: now.Add(module.cfg.CeremonyTTL), CreatedAt: now,
	}
	if err := module.store.CreateCeremony(ctx, ceremony); err != nil {
		return BeginCeremonyResult{}, err
	}
	return BeginCeremonyResult{
		CeremonyID: ceremony.ID, ExpiresAt: ceremony.ExpiresAt, Options: options.JSON,
	}, nil
}

type FinishPasskeyAuthenticationRequest struct {
	CeremonyID string
	Response   []byte
	UserAgent  string
	IP         string
}

type AuthenticationResult struct {
	SessionID      string
	IdentityID     string
	Authentication Context
}

func (module *Module) FinishPasskeyAuthentication(
	ctx context.Context,
	request FinishPasskeyAuthenticationRequest,
) (AuthenticationResult, error) {
	ceremony, err := module.loadCeremony(ctx, request.CeremonyID, CeremonyPasskeyAuthentication)
	if err != nil {
		return AuthenticationResult{}, err
	}
	user, credential, err := module.verifier.FinishDiscoverableAuthentication(
		CeremonyMaterial{
			ChallengeDigest: ceremony.ChallengeDigest, LibraryState: ceremony.LibraryState,
		},
		request.Response,
		func(handle []byte) (PasskeyUser, error) {
			return module.store.GetPasskeyUserByHandle(ctx, handle)
		},
	)
	if err != nil {
		module.recordFailure(ctx, ceremony)
		return AuthenticationResult{}, ErrCeremonyInvalid
	}
	now := module.now().UTC()
	auth := Passkey(uuid.NewString(), now, credential.ID, credential.UserVerified)
	session := Session{
		ID: uuid.NewString(), IdentityID: user.IdentityID,
		CreatedAt: now, LastSeen: now, UserAgent: request.UserAgent, IP: request.IP,
		ExpiresAt: now.Add(module.cfg.SessionTTL), Authentication: auth,
	}
	if err := module.store.CompletePasskeyAuthentication(
		ctx, ceremony, credential, session, module.cfg.SessionTTL,
	); err != nil {
		return AuthenticationResult{}, err
	}
	if module.cache != nil {
		_ = module.cache.CreateSession(ctx, session, module.cfg.SessionTTL)
	}
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventPasskeyLogin, IdentityID: user.IdentityID,
		CredentialID: credential.ID, SessionID: session.ID, OccurredAt: now,
		Detail: map[string]any{
			"userVerified": credential.UserVerified,
			"backupState":  credential.BackupState,
		},
	})
	return AuthenticationResult{
		SessionID: session.ID, IdentityID: user.IdentityID, Authentication: auth,
	}, nil
}

func (module *Module) ListPasskeys(ctx context.Context, identityID string) ([]PasskeyCredential, error) {
	return module.store.ListPasskeys(ctx, identityID)
}

func (module *Module) RenamePasskey(
	ctx context.Context,
	identityID, credentialID, label string,
) (PasskeyCredential, error) {
	credential, err := module.store.RenamePasskey(
		ctx, identityID, credentialID, label, module.now().UTC(),
	)
	if err == nil {
		module.recordEvent(ctx, SecurityEvent{
			Kind: EventPasskeyRenamed, IdentityID: identityID,
			CredentialID: credentialID, Label: label,
		})
	}
	return credential, err
}

// RevokePasskey permanently makes a credential unusable. The store performs
// the alternative-authenticator check while holding the identity row lock, so
// concurrent credential removals cannot strand the account.
func (module *Module) RevokePasskey(
	ctx context.Context,
	identityID, credentialID string,
) error {
	err := module.store.RevokePasskey(
		ctx, identityID, credentialID, "user_removed", module.now().UTC(),
	)
	if err == nil {
		module.recordEvent(ctx, SecurityEvent{
			Kind: EventPasskeyRevoked, IdentityID: identityID,
			CredentialID: credentialID,
		})
	}
	return err
}

func (module *Module) loadCeremony(
	ctx context.Context,
	id string,
	kind CeremonyKind,
) (Ceremony, error) {
	ceremony, err := module.store.GetCeremony(ctx, id)
	if err != nil || ceremony.Kind != kind {
		return Ceremony{}, ErrCeremonyInvalid
	}
	if ceremony.ConsumedAt != nil {
		return Ceremony{}, ErrCeremonyConsumed
	}
	if !ceremony.ExpiresAt.After(module.now().UTC()) || ceremony.FailedAttempts >= 5 {
		return Ceremony{}, ErrCeremonyExpired
	}
	return ceremony, nil
}

func (module *Module) recordFailure(ctx context.Context, ceremony Ceremony) {
	_ = module.store.RecordCeremonyFailure(ctx, ceremony.ID, 5)
}

func challengeDigest(challenge string) []byte {
	sum := sha256.Sum256([]byte(challenge))
	return sum[:]
}
