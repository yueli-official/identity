//go:build integration

package dao_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/dao"
	"github.com/yueli-official/identity/internal/repo"
)

func TestPGPasskeyLifecycleAndAuthenticationSession(t *testing.T) {
	db := newDB(t)
	store := dao.NewPG(db)
	ctx := context.Background()

	identity, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email:        "passkey-" + uuid.NewString() + "@pg.test",
		PasswordHash: "password-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	handleCandidate := []byte("01234567890123456789012345678901")
	handle, err := store.GetOrCreatePasskeyUser(ctx, identity.ID, handleCandidate)
	if err != nil {
		t.Fatalf("GetOrCreatePasskeyUser() error = %v", err)
	}
	if string(handle) != string(handleCandidate) {
		t.Fatalf("user handle = %x, want %x", handle, handleCandidate)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	bindingSession := authentication.Session{
		ID: uuid.NewString(), IdentityID: identity.ID,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour),
		Authentication: authentication.Password(uuid.NewString(), now),
	}
	if err := store.CreateSession(ctx, bindingSession, time.Hour); err != nil {
		t.Fatalf("CreateSession(binding) error = %v", err)
	}
	registration := authentication.Ceremony{
		ID: uuid.NewString(), Kind: authentication.CeremonyPasskeyRegistration,
		IdentityID: identity.ID, SessionID: bindingSession.ID,
		ChallengeDigest: make([]byte, 32), LibraryState: []byte(`{}`),
		ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now,
	}
	if err := store.CreateCeremony(ctx, registration); err != nil {
		t.Fatalf("CreateCeremony(registration) error = %v", err)
	}
	credential := authentication.PasskeyCredential{
		ID: uuid.NewString(), IdentityID: identity.ID, RPID: "account.example.test",
		CredentialID: []byte("credential-" + uuid.NewString()),
		PublicKey:    []byte("public-key"), PublicKeyAlgorithm: -7,
		Transports: []string{"internal"}, Attachment: "platform",
		Flags: 5, UserVerifiedAtRegistration: true,
		Status: "active", Label: "Test device", Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CompletePasskeyRegistration(ctx, registration, credential); err != nil {
		t.Fatalf("CompletePasskeyRegistration() error = %v", err)
	}
	if err := store.CompletePasskeyRegistration(ctx, registration, credential); !errors.Is(err, authentication.ErrCeremonyConsumed) {
		t.Fatalf("registration replay error = %v, want ErrCeremonyConsumed", err)
	}
	if count, err := store.CountActivePasskeys(ctx, identity.ID); err != nil || count != 1 {
		t.Fatalf("CountActivePasskeys() = %d, %v; want 1", count, err)
	}

	login := authentication.Ceremony{
		ID: uuid.NewString(), Kind: authentication.CeremonyPasskeyAuthentication,
		ChallengeDigest: make([]byte, 32), LibraryState: []byte(`{}`),
		ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now,
	}
	if err := store.CreateCeremony(ctx, login); err != nil {
		t.Fatalf("CreateCeremony(authentication) error = %v", err)
	}
	session := authentication.Session{
		ID: uuid.NewString(), IdentityID: identity.ID,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour),
		Authentication: authentication.Passkey(
			uuid.NewString(), now, credential.ID, true,
		),
	}
	credential.UserVerified = true
	if err := store.CompletePasskeyAuthentication(
		ctx, login, credential, session, time.Hour,
	); err != nil {
		t.Fatalf("CompletePasskeyAuthentication() error = %v", err)
	}
	storedSession, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if storedSession.Authentication.EventID != session.Authentication.EventID ||
		storedSession.Authentication.Level != authentication.LevelAAL2 {
		t.Fatalf("stored authentication = %+v", storedSession.Authentication)
	}

	renamed, err := store.RenamePasskey(
		ctx, identity.ID, credential.ID, "Laptop", now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("RenamePasskey() error = %v", err)
	}
	if renamed.Label != "Laptop" || renamed.Version != 3 {
		t.Fatalf("renamed passkey = %+v", renamed)
	}
	if err := store.RevokePasskey(
		ctx, identity.ID, credential.ID, "test", now.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("RevokePasskey() error = %v", err)
	}
	if count, err := store.CountActivePasskeys(ctx, identity.ID); err != nil || count != 0 {
		t.Fatalf("active passkeys after revoke = %d, %v; want 0", count, err)
	}
}

func TestPGCredentialRemovalCannotStrandOAuthOnlyAccount(t *testing.T) {
	db := newDB(t)
	store := dao.NewPG(db)
	ctx := context.Background()

	identity, err := store.CreateOAuthIdentity(ctx, repo.NewOAuthIdentityInput{
		Email:         "passkey-last-" + uuid.NewString() + "@pg.test",
		EmailVerified: true, Provider: "google", ProviderUID: uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	bindingSession := authentication.Session{
		ID: uuid.NewString(), IdentityID: identity.ID,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour),
		Authentication: authentication.Federated(uuid.NewString(), now, "google"),
	}
	if err := store.CreateSession(ctx, bindingSession, time.Hour); err != nil {
		t.Fatal(err)
	}
	handle := []byte("abcdefghijklmnopqrstuvwxyzABCDEF")
	if _, err := store.GetOrCreatePasskeyUser(ctx, identity.ID, handle); err != nil {
		t.Fatal(err)
	}
	ceremony := authentication.Ceremony{
		ID: uuid.NewString(), Kind: authentication.CeremonyPasskeyRegistration,
		IdentityID: identity.ID, SessionID: bindingSession.ID,
		ChallengeDigest: make([]byte, 32), LibraryState: []byte(`{}`),
		ExpiresAt: now.Add(time.Minute), CreatedAt: now,
	}
	if err := store.CreateCeremony(ctx, ceremony); err != nil {
		t.Fatal(err)
	}
	credential := authentication.PasskeyCredential{
		ID: uuid.NewString(), IdentityID: identity.ID, RPID: "account.example.test",
		CredentialID: []byte("credential-" + uuid.NewString()),
		PublicKey:    []byte("public-key"), PublicKeyAlgorithm: -7,
		Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CompletePasskeyRegistration(ctx, ceremony, credential); err != nil {
		t.Fatal(err)
	}

	removed, err := store.DeleteOAuthCredential(ctx, identity.ID, "google")
	if err != nil || !removed {
		t.Fatalf("DeleteOAuthCredential() = %v, %v; passkey is an alternative", removed, err)
	}
	if err := store.RevokePasskey(
		ctx, identity.ID, credential.ID, "test", now,
	); !errors.Is(err, authentication.ErrLastAuthenticator) {
		t.Fatalf("last passkey revoke error = %v, want ErrLastAuthenticator", err)
	}
}
