package repo_test

import (
	"context"
	"errors"
	"testing"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

func TestMemoryIdentityCreateAndGet(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	id, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "a@b.com", DisplayName: "A", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.GetByEmail(ctx, "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id.ID {
		t.Fatalf("id mismatch")
	}
	if _, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "a@b.com", PasswordHash: "h"}); err != repo.ErrEmailTaken {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestMemoryGetProfile(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	id, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "profile@test.com", DisplayName: "Profile User", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	prof, err := m.GetProfile(ctx, id.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if prof.DisplayName != "Profile User" {
		t.Fatalf("want DisplayName 'Profile User', got %q", prof.DisplayName)
	}
	if _, err := m.GetProfile(ctx, "nonexistent-id"); err != repo.ErrIdentityMissing {
		t.Fatalf("want ErrIdentityMissing for unknown id, got %v", err)
	}
}

func TestMemoryClientRepo(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	c := model.OIDCClient{
		ID:            "client-1",
		Public:        false,
		SecretHash:    "hash",
		RedirectURIs:  []string{"https://example.com/callback"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile"},
	}
	m.SetClient(c)
	got, err := m.GetClient(ctx, "client-1")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if got.ID != "client-1" {
		t.Fatalf("want client ID 'client-1', got %q", got.ID)
	}
	if got.SecretHash != "hash" {
		t.Fatalf("want SecretHash 'hash', got %q", got.SecretHash)
	}
	if _, err := m.GetClient(ctx, "nonexistent-client"); err != repo.ErrClientNotFound {
		t.Fatalf("want ErrClientNotFound, got %v", err)
	}
}

func TestMemorySigningKeyRepo(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	k := model.SigningKey{
		KID:        "key-1",
		Alg:        "RS256",
		PrivatePEM: "private-pem-data",
		PublicPEM:  "public-pem-data",
		Status:     model.KeyActive,
	}
	if err := m.InsertKey(ctx, k); err != nil {
		t.Fatalf("InsertKey: %v", err)
	}
	active, err := m.GetActiveKey(ctx)
	if err != nil {
		t.Fatalf("GetActiveKey: %v", err)
	}
	if active.KID != "key-1" {
		t.Fatalf("want KID 'key-1', got %q", active.KID)
	}
	keys, err := m.ListPublicKeys(ctx)
	if err != nil {
		t.Fatalf("ListPublicKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want 1 key, got %d", len(keys))
	}
	// No active key scenario
	m2 := repo.NewMemory()
	if _, err := m2.GetActiveKey(ctx); err != repo.ErrNoActiveKey {
		t.Fatalf("want ErrNoActiveKey, got %v", err)
	}
}

func TestMemory_OAuth_CreateThenGetByProviderUID(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, err := m.CreateOAuthIdentity(ctx, repo.NewOAuthIdentityInput{
		Email: "g@example.com", EmailVerified: true, DisplayName: "G",
		Provider: "google", ProviderUID: "uid-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.GetByProviderUID(ctx, "google", "uid-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id.ID || got.Email != "g@example.com" || !got.EmailVerified {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestMemory_OAuth_GetByProviderUID_Missing(t *testing.T) {
	m := repo.NewMemory()
	_, err := m.GetByProviderUID(context.Background(), "google", "nope")
	if !errors.Is(err, repo.ErrIdentityMissing) {
		t.Fatalf("want ErrIdentityMissing, got %v", err)
	}
}

func TestMemory_OAuth_LinkExistingIdentity(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	base, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "a@example.com", DisplayName: "A", PasswordHash: "h"})
	if err := m.LinkOAuthCredential(ctx, base.ID, "google", "uid-2", "a@example.com", true); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetByProviderUID(ctx, "google", "uid-2")
	if got.ID != base.ID {
		t.Fatalf("link should resolve to base identity, got %s", got.ID)
	}
}

func TestMemorySessionLifecycle(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	s := model.Session{ID: "s1", IdentityID: "u1"}
	if err := m.CreateSession(ctx, s, 0); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetSession(ctx, "s1")
	if err != nil || got.IdentityID != "u1" {
		t.Fatalf("get session: %v %#v", err, got)
	}
	list, _ := m.ListSessionsByIdentity(ctx, "u1")
	if len(list) != 1 {
		t.Fatalf("want 1 session, got %d", len(list))
	}
	_ = m.DeleteSession(ctx, "s1")
	if _, err := m.GetSession(ctx, "s1"); err != repo.ErrSessionNotFound {
		t.Fatalf("want ErrSessionNotFound, got %v", err)
	}
}
