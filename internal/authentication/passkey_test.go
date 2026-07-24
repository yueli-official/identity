package authentication

import (
	"context"
	"errors"
	"testing"
	"time"
)

type passkeyTestStore struct {
	user        PasskeyUser
	ceremonies  map[string]Ceremony
	credentials map[string]PasskeyCredential
	sessions    map[string]Session
	cache       map[string]Session
	failures    int
}

func newPasskeyTestStore() *passkeyTestStore {
	return &passkeyTestStore{
		user: PasskeyUser{
			IdentityID: "identity-1",
			UserHandle: []byte("01234567890123456789012345678901"),
		},
		ceremonies:  map[string]Ceremony{},
		credentials: map[string]PasskeyCredential{},
		sessions:    map[string]Session{},
		cache:       map[string]Session{},
	}
}

func (store *passkeyTestStore) GetOrCreatePasskeyUser(
	_ context.Context,
	identityID string,
	candidate []byte,
) ([]byte, error) {
	if len(store.user.UserHandle) == 0 {
		store.user.IdentityID = identityID
		store.user.UserHandle = append([]byte(nil), candidate...)
	}
	return append([]byte(nil), store.user.UserHandle...), nil
}

func (store *passkeyTestStore) GetPasskeyUserByIdentity(
	_ context.Context,
	identityID string,
) (PasskeyUser, error) {
	if store.user.IdentityID != identityID {
		return PasskeyUser{}, ErrPasskeyNotFound
	}
	user := store.user
	user.Credentials = store.activeCredentials()
	return user, nil
}

func (store *passkeyTestStore) GetPasskeyUserByHandle(
	_ context.Context,
	handle []byte,
) (PasskeyUser, error) {
	if string(store.user.UserHandle) != string(handle) {
		return PasskeyUser{}, ErrPasskeyNotFound
	}
	user := store.user
	user.Credentials = store.activeCredentials()
	return user, nil
}

func (store *passkeyTestStore) ListPasskeys(
	_ context.Context,
	identityID string,
) ([]PasskeyCredential, error) {
	if store.user.IdentityID != identityID {
		return nil, ErrPasskeyNotFound
	}
	return store.activeCredentials(), nil
}

func (store *passkeyTestStore) RenamePasskey(
	_ context.Context,
	identityID, credentialID, label string,
	now time.Time,
) (PasskeyCredential, error) {
	credential, ok := store.credentials[credentialID]
	if !ok || credential.IdentityID != identityID || credential.Status == "revoked" {
		return PasskeyCredential{}, ErrPasskeyNotFound
	}
	credential.Label = label
	credential.UpdatedAt = now
	credential.Version++
	store.credentials[credentialID] = credential
	return credential, nil
}

func (store *passkeyTestStore) RevokePasskey(
	_ context.Context,
	identityID, credentialID, _ string,
	now time.Time,
) error {
	credential, ok := store.credentials[credentialID]
	if !ok || credential.IdentityID != identityID || credential.Status == "revoked" {
		return ErrPasskeyNotFound
	}
	credential.Status = "revoked"
	credential.UpdatedAt = now
	credential.Version++
	store.credentials[credentialID] = credential
	return nil
}

func (store *passkeyTestStore) CreateCeremony(_ context.Context, ceremony Ceremony) error {
	store.ceremonies[ceremony.ID] = ceremony
	return nil
}

func (store *passkeyTestStore) GetCeremony(_ context.Context, id string) (Ceremony, error) {
	ceremony, ok := store.ceremonies[id]
	if !ok {
		return Ceremony{}, ErrCeremonyInvalid
	}
	return ceremony, nil
}

func (store *passkeyTestStore) RecordCeremonyFailure(_ context.Context, id string, max int) error {
	ceremony := store.ceremonies[id]
	if ceremony.FailedAttempts < max {
		ceremony.FailedAttempts++
		store.failures++
	}
	store.ceremonies[id] = ceremony
	return nil
}

func (store *passkeyTestStore) CompletePasskeyRegistration(
	_ context.Context,
	ceremony Ceremony,
	credential PasskeyCredential,
) error {
	current := store.ceremonies[ceremony.ID]
	if current.ConsumedAt != nil {
		return ErrCeremonyConsumed
	}
	now := credential.CreatedAt
	current.ConsumedAt = &now
	store.ceremonies[ceremony.ID] = current
	store.credentials[credential.ID] = credential
	return nil
}

func (store *passkeyTestStore) CompletePasskeyAuthentication(
	_ context.Context,
	ceremony Ceremony,
	credential PasskeyCredential,
	session Session,
	_ time.Duration,
) error {
	current := store.ceremonies[ceremony.ID]
	if current.ConsumedAt != nil {
		return ErrCeremonyConsumed
	}
	now := session.Authentication.AuthenticatedAt
	current.ConsumedAt = &now
	store.ceremonies[ceremony.ID] = current
	stored := store.credentials[credential.ID]
	credential.IdentityID = stored.IdentityID
	credential.Status = stored.Status
	credential.Version = stored.Version + 1
	store.credentials[credential.ID] = credential
	store.sessions[session.ID] = session
	return nil
}

func (store *passkeyTestStore) CreateSession(
	_ context.Context,
	session Session,
	_ time.Duration,
) error {
	store.cache[session.ID] = session
	return nil
}

func (store *passkeyTestStore) activeCredentials() []PasskeyCredential {
	out := make([]PasskeyCredential, 0, len(store.credentials))
	for _, credential := range store.credentials {
		if credential.Status == "active" {
			out = append(out, credential)
		}
	}
	return out
}

type passkeyTestVerifier struct {
	store          *passkeyTestStore
	registrationOK bool
	loginOK        bool
	userVerified   bool
}

func (verifier *passkeyTestVerifier) BeginRegistration(PasskeyUser) (
	CeremonyMaterial,
	BrowserOptions,
	error,
) {
	return CeremonyMaterial{
		ChallengeDigest: make([]byte, 32),
		LibraryState:    []byte(`{"registration":true}`),
	}, BrowserOptions{JSON: []byte(`{"publicKey":{"challenge":"registration"}}`)}, nil
}

func (verifier *passkeyTestVerifier) FinishRegistration(
	user PasskeyUser,
	_ CeremonyMaterial,
	_ []byte,
) (PasskeyCredential, error) {
	if !verifier.registrationOK {
		return PasskeyCredential{}, errors.New("invalid registration")
	}
	return PasskeyCredential{
		IdentityID:                 user.IdentityID,
		RPID:                       "account.example.test",
		CredentialID:               []byte("credential"),
		PublicKey:                  []byte("public-key"),
		UserVerifiedAtRegistration: true,
	}, nil
}

func (verifier *passkeyTestVerifier) BeginDiscoverableAuthentication() (
	CeremonyMaterial,
	BrowserOptions,
	error,
) {
	return CeremonyMaterial{
		ChallengeDigest: make([]byte, 32),
		LibraryState:    []byte(`{"authentication":true}`),
	}, BrowserOptions{JSON: []byte(`{"publicKey":{"challenge":"authentication"}}`)}, nil
}

func (verifier *passkeyTestVerifier) FinishDiscoverableAuthentication(
	_ CeremonyMaterial,
	_ []byte,
	resolve func([]byte) (PasskeyUser, error),
) (PasskeyUser, PasskeyCredential, error) {
	if !verifier.loginOK {
		return PasskeyUser{}, PasskeyCredential{}, errors.New("invalid assertion")
	}
	user, err := resolve(verifier.store.user.UserHandle)
	if err != nil {
		return PasskeyUser{}, PasskeyCredential{}, err
	}
	credential := user.Credentials[0]
	credential.UserVerified = verifier.userVerified
	return user, credential, nil
}

func TestPasskeyRegistrationBindsSessionAndConsumesCeremony(t *testing.T) {
	ctx := context.Background()
	store := newPasskeyTestStore()
	verifier := &passkeyTestVerifier{store: store, registrationOK: true}
	module, err := NewModule(store, store, verifier, ModuleConfig{
		SessionTTL: time.Hour, CeremonyTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}
	now := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	module.now = func() time.Time { return now }

	begin, err := module.BeginPasskeyRegistration(ctx, BeginPasskeyRegistrationRequest{
		IdentityID: "identity-1", SessionID: "session-1",
		Name: "user@example.test", DisplayName: "User",
	})
	if err != nil {
		t.Fatalf("BeginPasskeyRegistration() error = %v", err)
	}
	if got := store.ceremonies[begin.CeremonyID]; got.SessionID != "session-1" ||
		got.IdentityID != "identity-1" || !got.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("stored ceremony = %+v", got)
	}

	_, err = module.FinishPasskeyRegistration(ctx, FinishPasskeyRegistrationRequest{
		CeremonyID: begin.CeremonyID, SessionID: "another-session", Response: []byte("ok"),
	})
	if !errors.Is(err, ErrCeremonyInvalid) {
		t.Fatalf("wrong-session finish error = %v, want ErrCeremonyInvalid", err)
	}

	credential, err := module.FinishPasskeyRegistration(ctx, FinishPasskeyRegistrationRequest{
		CeremonyID: begin.CeremonyID, SessionID: "session-1",
		Label: "Phone", Response: []byte("ok"),
	})
	if err != nil {
		t.Fatalf("FinishPasskeyRegistration() error = %v", err)
	}
	if credential.ID == "" || credential.Label != "Phone" || credential.Status != "active" {
		t.Fatalf("credential = %+v", credential)
	}

	_, err = module.FinishPasskeyRegistration(ctx, FinishPasskeyRegistrationRequest{
		CeremonyID: begin.CeremonyID, SessionID: "session-1", Response: []byte("replay"),
	})
	if !errors.Is(err, ErrCeremonyConsumed) {
		t.Fatalf("replayed finish error = %v, want ErrCeremonyConsumed", err)
	}
}

func TestPasskeyLoginPersistsActualUVContextAndWarmsCache(t *testing.T) {
	ctx := context.Background()
	store := newPasskeyTestStore()
	store.credentials["credential-1"] = PasskeyCredential{
		ID: "credential-1", IdentityID: "identity-1", Status: "active", Version: 1,
	}
	verifier := &passkeyTestVerifier{
		store: store, loginOK: true, userVerified: true,
	}
	module, err := NewModule(store, store, verifier, ModuleConfig{
		SessionTTL: 24 * time.Hour, CeremonyTTL: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}
	now := time.Date(2026, 7, 24, 11, 0, 0, 0, time.UTC)
	module.now = func() time.Time { return now }

	begin, err := module.BeginPasskeyAuthentication(ctx)
	if err != nil {
		t.Fatalf("BeginPasskeyAuthentication() error = %v", err)
	}
	result, err := module.FinishPasskeyAuthentication(ctx, FinishPasskeyAuthenticationRequest{
		CeremonyID: begin.CeremonyID, Response: []byte("ok"),
		UserAgent: "Browser", IP: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("FinishPasskeyAuthentication() error = %v", err)
	}
	if result.IdentityID != "identity-1" ||
		result.Authentication.Level != LevelAAL2 ||
		!result.Authentication.UserVerified ||
		!result.Authentication.PhishingResistant ||
		!result.Authentication.AuthenticatedAt.Equal(now) {
		t.Fatalf("authentication result = %+v", result)
	}
	if _, ok := store.sessions[result.SessionID]; !ok {
		t.Fatal("durable session was not stored")
	}
	if _, ok := store.cache[result.SessionID]; !ok {
		t.Fatal("session cache was not warmed")
	}
}

func TestPasskeyCeremonyExpiryAndFailureLimit(t *testing.T) {
	ctx := context.Background()
	store := newPasskeyTestStore()
	verifier := &passkeyTestVerifier{store: store}
	module, err := NewModule(store, store, verifier, ModuleConfig{CeremonyTTL: time.Minute})
	if err != nil {
		t.Fatalf("NewModule() error = %v", err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	module.now = func() time.Time { return now }

	begin, err := module.BeginPasskeyAuthentication(ctx)
	if err != nil {
		t.Fatalf("BeginPasskeyAuthentication() error = %v", err)
	}
	for range 5 {
		_, err = module.FinishPasskeyAuthentication(ctx, FinishPasskeyAuthenticationRequest{
			CeremonyID: begin.CeremonyID, Response: []byte("invalid"),
		})
		if !errors.Is(err, ErrCeremonyInvalid) {
			t.Fatalf("failed finish error = %v, want ErrCeremonyInvalid", err)
		}
	}
	_, err = module.FinishPasskeyAuthentication(ctx, FinishPasskeyAuthenticationRequest{
		CeremonyID: begin.CeremonyID, Response: []byte("invalid"),
	})
	if !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("failure-limited finish error = %v, want ErrCeremonyExpired", err)
	}

	expiring, err := module.BeginPasskeyAuthentication(ctx)
	if err != nil {
		t.Fatalf("BeginPasskeyAuthentication() error = %v", err)
	}
	module.now = func() time.Time { return now.Add(time.Minute) }
	_, err = module.FinishPasskeyAuthentication(ctx, FinishPasskeyAuthenticationRequest{
		CeremonyID: expiring.CeremonyID, Response: []byte("ok"),
	})
	if !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("expired finish error = %v, want ErrCeremonyExpired", err)
	}
}

var (
	_ PasskeyStore     = (*passkeyTestStore)(nil)
	_ SessionCache     = (*passkeyTestStore)(nil)
	_ WebAuthnVerifier = (*passkeyTestVerifier)(nil)
)
