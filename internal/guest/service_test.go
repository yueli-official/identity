package guest_test

import (
	"context"
	"testing"
	"time"

	foundationauth "github.com/yueli-official/foundation/go/auth"
	"github.com/yueli-official/identity/internal/guest"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/oidc"
	"github.com/yueli-official/identity/internal/repo"
)

func fixture(t *testing.T) (*guest.Service, *repo.Memory, *oidc.Manager, time.Time) {
	t.Helper()
	store := repo.NewMemory()
	store.SetClient(model.OIDCClient{
		ID:        "consumer-web",
		Audiences: []string{"consumer-web", "asset-api"},
	})
	keys, err := oidc.NewManager(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	service := guest.New(store, store, store, keys, guest.Config{
		Issuer:         "https://identity.test",
		MaxSessionTTL:  30 * 24 * time.Hour,
		AccessTokenTTL: 10 * time.Minute,
		Now:            func() time.Time { return now },
	})
	return service, store, keys, now
}

func TestCreateUsesRequestedTTLAndReturnsEffectiveExpiry(t *testing.T) {
	service, _, _, now := fixture(t)
	created, err := service.Create(context.Background(), "consumer-web", 10*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if created.EffectiveTTL != 10*24*time.Hour || !created.ExpiresAt.Equal(now.Add(10*24*time.Hour)) {
		t.Fatalf("created = %+v", created)
	}
	clamped, err := service.Create(context.Background(), "consumer-web", 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if clamped.EffectiveTTL != 30*24*time.Hour {
		t.Fatalf("effective ttl = %s", clamped.EffectiveTTL)
	}
}

func TestTokenIsShortLivedGuestAndResourceBound(t *testing.T) {
	service, _, keys, now := fixture(t)
	created, err := service.Create(context.Background(), "consumer-web", 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Token(context.Background(), "consumer-web", created.SessionToken, "asset-api")
	if err != nil {
		t.Fatal(err)
	}
	if issued.ExpiresIn != 10*time.Minute || issued.AccessToken == "" {
		t.Fatalf("issued = %+v", issued)
	}
	verifier, err := foundationauth.NewVerifier(foundationauth.Config{
		Keys: keys, Issuer: "https://identity.test", Audiences: []string{"asset-api"}, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	subjectKind, _ := principal.Claim("subject_kind")
	if principal.Subject != created.SubjectID || principal.ClientID != "consumer-web" || subjectKind != "guest" {
		t.Fatalf("principal = %+v", principal)
	}
	if _, err := service.Token(context.Background(), "consumer-web", created.SessionToken, "commerce-api"); err == nil {
		t.Fatal("unregistered audience was accepted")
	}
}

func TestClaimIsIdempotentForOneUserAndRejectsAnother(t *testing.T) {
	service, store, keys, now := fixture(t)
	firstUser, err := store.CreateIdentityWithProfile(context.Background(), repo.NewIdentityInput{
		Email: "first@example.com", DisplayName: "First", PasswordHash: "hash", Roles: []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}
	secondUser, err := store.CreateIdentityWithProfile(context.Background(), repo.NewIdentityInput{
		Email: "second@example.com", DisplayName: "Second", PasswordHash: "hash", Roles: []string{"user"},
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), "consumer-web", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Claim(context.Background(), "consumer-web", created.SessionToken, firstUser.UserKey)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Claim(context.Background(), "consumer-web", created.SessionToken, firstUser.UserKey)
	if err != nil {
		t.Fatal(err)
	}
	if first.SubjectID != second.SubjectID || first.UserKey != second.UserKey {
		t.Fatalf("claim is not idempotent: %+v %+v", first, second)
	}
	assertion, err := service.ClaimForAudience(context.Background(), "consumer-web", created.SessionToken, first.UserKey, "asset-api")
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := foundationauth.NewVerifier(foundationauth.Config{Keys: keys, Issuer: "https://identity.test", Audiences: []string{"asset-api"}, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(context.Background(), assertion.ClaimToken)
	if err != nil {
		t.Fatal(err)
	}
	guestSubject, _ := principal.Claim("guest_subject")
	if principal.Subject != first.UserKey || !principal.HasScope("guest:claim") || guestSubject != created.SubjectID {
		t.Fatalf("claim principal = %+v", principal)
	}
	if _, err := service.Claim(context.Background(), "consumer-web", created.SessionToken, secondUser.UserKey); err == nil {
		t.Fatal("guest session was claimed by a second user")
	}
}
