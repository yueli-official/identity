package guest_test

import (
	"context"
	"testing"
	"time"

	"platform/gokit/authjwt"
	"platform/services/identity/internal/guest"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

func fixture(t *testing.T) (*guest.Service, *repo.Memory, *oidc.Manager, time.Time) {
	t.Helper()
	store := repo.NewMemory()
	store.SetClient(model.OIDCClient{
		ID:        "gallery-main-web",
		Audiences: []string{"gallery-main-web", "asset-api"},
	})
	keys, err := oidc.NewManager(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	service := guest.New(store, store, keys, guest.Config{
		Issuer:         "https://identity.test",
		MaxSessionTTL:  30 * 24 * time.Hour,
		AccessTokenTTL: 10 * time.Minute,
		Now:            func() time.Time { return now },
	})
	return service, store, keys, now
}

func TestCreateUsesRequestedTTLAndReturnsEffectiveExpiry(t *testing.T) {
	service, _, _, now := fixture(t)
	created, err := service.Create(context.Background(), "gallery-main-web", 10*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if created.EffectiveTTL != 10*24*time.Hour || !created.ExpiresAt.Equal(now.Add(10*24*time.Hour)) {
		t.Fatalf("created = %+v", created)
	}
	clamped, err := service.Create(context.Background(), "gallery-main-web", 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if clamped.EffectiveTTL != 30*24*time.Hour {
		t.Fatalf("effective ttl = %s", clamped.EffectiveTTL)
	}
}

func TestTokenIsShortLivedGuestAndResourceBound(t *testing.T) {
	service, _, keys, now := fixture(t)
	created, err := service.Create(context.Background(), "gallery-main-web", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Token(context.Background(), "gallery-main-web", created.SessionToken, "asset-api")
	if err != nil {
		t.Fatal(err)
	}
	if issued.ExpiresIn != 10*time.Minute || issued.AccessToken == "" {
		t.Fatalf("issued = %+v", issued)
	}
	verifier, err := authjwt.NewVerifier(authjwt.VerifierConfig{
		Keys: keys, Issuer: "https://identity.test", Audience: "asset-api", Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != created.SubjectID || principal.ClientID != "gallery-main-web" || principal.Claims["subject_kind"] != "guest" {
		t.Fatalf("principal = %+v", principal)
	}
	if _, err := service.Token(context.Background(), "gallery-main-web", created.SessionToken, "commerce-api"); err == nil {
		t.Fatal("unregistered audience was accepted")
	}
}

func TestClaimIsIdempotentForOneUserAndRejectsAnother(t *testing.T) {
	service, _, keys, now := fixture(t)
	created, err := service.Create(context.Background(), "gallery-main-web", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Claim(context.Background(), "gallery-main-web", created.SessionToken, "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Claim(context.Background(), "gallery-main-web", created.SessionToken, "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if first.SubjectID != second.SubjectID || first.UserID != second.UserID {
		t.Fatalf("claim is not idempotent: %+v %+v", first, second)
	}
	assertion, err := service.ClaimForAudience(context.Background(), "gallery-main-web", created.SessionToken, first.UserID, "asset-api")
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := authjwt.NewVerifier(authjwt.VerifierConfig{Keys: keys, Issuer: "https://identity.test", Audience: "asset-api", Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(context.Background(), assertion.ClaimToken)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Subject != first.UserID || !principal.HasScope("guest:claim") || principal.Claims["guest_subject"] != created.SubjectID {
		t.Fatalf("claim principal = %+v", principal)
	}
	if _, err := service.Claim(context.Background(), "gallery-main-web", created.SessionToken, "22222222-2222-4222-8222-222222222222"); err == nil {
		t.Fatal("guest session was claimed by a second user")
	}
}
