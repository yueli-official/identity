package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestMemory_Verification_ConsumeOnce(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "v@example.com", DisplayName: "V", PasswordHash: "h"})
	in := repo.NewVerificationInput{IdentityID: id.ID, Email: "v@example.com", Purpose: repo.PurposeVerifyEmail, TokenHash: "hash1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := m.CreateVerification(ctx, in); err != nil {
		t.Fatal(err)
	}
	rec, err := m.ConsumeVerification(ctx, "hash1", repo.PurposeVerifyEmail)
	if err != nil || rec.IdentityID != id.ID {
		t.Fatalf("first consume: %+v %v", rec, err)
	}
	if _, err := m.ConsumeVerification(ctx, "hash1", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("second consume must fail single-use, got %v", err)
	}
}

func TestMemory_Verification_WrongPurpose(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "p@example.com", DisplayName: "P", PasswordHash: "h"})
	_ = m.CreateVerification(ctx, repo.NewVerificationInput{IdentityID: id.ID, Email: "p@example.com", Purpose: repo.PurposePasswordReset, TokenHash: "h2", ExpiresAt: time.Now().Add(time.Hour)})
	if _, err := m.ConsumeVerification(ctx, "h2", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("purpose mismatch must fail, got %v", err)
	}
}

func TestMemory_Verification_Expired(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "e@example.com", DisplayName: "E", PasswordHash: "h"})
	_ = m.CreateVerification(ctx, repo.NewVerificationInput{IdentityID: id.ID, Email: "e@example.com", Purpose: repo.PurposeVerifyEmail, TokenHash: "h3", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, err := m.ConsumeVerification(ctx, "h3", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("expired must fail, got %v", err)
	}
}

func TestMemory_SetEmailVerified_And_UpdatePassword(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "s@example.com", DisplayName: "S", PasswordHash: "old"})
	if err := m.SetEmailVerified(ctx, id.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetByID(ctx, id.ID)
	if !got.EmailVerified {
		t.Fatal("email_verified not set")
	}
	if err := m.UpdatePasswordHash(ctx, id.ID, "newhash"); err != nil {
		t.Fatal(err)
	}
	h, _ := m.GetPasswordHash(ctx, id.ID)
	if h != "newhash" {
		t.Fatalf("password not updated: %q", h)
	}
}

func TestMemory_Roles_GrantGetRevoke(t *testing.T) {
	m := repo.NewMemory()
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "r@example.com", DisplayName: "R", PasswordHash: "h"})
	if r, _ := m.GetRoles(ctx, id.ID); len(r) != 0 {
		t.Fatalf("new identity has no roles, got %v", r)
	}
	if err := m.GrantRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := m.GrantRole(ctx, id.ID, "user"); err != nil {
		t.Fatal(err)
	}
	if err := m.GrantRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal("grant must be idempotent", err)
	}
	roles, _ := m.GetRoles(ctx, id.ID)
	if len(roles) != 2 {
		t.Fatalf("want 2 roles, got %v", roles)
	}
	// GetRoles must return a sorted slice ("admin" < "user").
	if roles[0] != "admin" || roles[1] != "user" {
		t.Fatalf("roles must be sorted, got %v", roles)
	}
	if err := m.GrantRole(ctx, id.ID, "nope"); !errors.Is(err, repo.ErrUnknownRole) {
		t.Fatalf("unknown role must error, got %v", err)
	}
	if err := m.RevokeRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	roles, _ = m.GetRoles(ctx, id.ID)
	if len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("after revoke want [user], got %v", roles)
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
