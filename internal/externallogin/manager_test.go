package externallogin

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/oauthlogin"
)

const testSecretMaterial = "0123456789abcdef0123456789abcdef"

func TestManagerOwnsCatalogSecretLifecycleAndPolicies(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	manager, err := New(store, testSecretMaterial, "https://account.example.test")
	if err != nil {
		t.Fatal(err)
	}
	views, err := manager.List(ctx)
	if err != nil || len(views) != 2 {
		t.Fatalf("views=%+v err=%v", views, err)
	}
	google, err := manager.Save(ctx, SaveInput{ActorID: "admin-1", Key: "google", ClientID: "google-id", ClientSecret: "google-secret", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if !google.Configured || google.SecretVersion != 1 || google.RegistrationPolicy != oauthlogin.RegistrationVerifiedEmail {
		t.Fatalf("google view=%+v", google)
	}
	qq, err := manager.Save(ctx, SaveInput{ActorID: "admin-1", Key: "qq", ClientID: "qq-id", ClientSecret: "qq-secret", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if qq.RegistrationPolicy != oauthlogin.RegistrationExistingOnly {
		t.Fatalf("qq policy=%q", qq.RegistrationPolicy)
	}
	provider, policy, err := manager.Resolve(ctx, "qq")
	if err != nil || provider.Name() != "qq" || policy != oauthlogin.RegistrationExistingOnly {
		t.Fatalf("resolve=%v %q %v", provider, policy, err)
	}
	stored, err := store.Get(ctx, "qq")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ClientSecretCipher == "qq-secret" || stored.ClientSecretCipher == "" {
		t.Fatalf("secret was not encrypted: %q", stored.ClientSecretCipher)
	}
	rotated, err := manager.Save(ctx, SaveInput{ActorID: "admin-1", Key: "qq", ClientID: "qq-id", ClientSecret: "rotated", Enabled: true})
	if err != nil || rotated.SecretVersion != 2 {
		t.Fatalf("rotated=%+v err=%v", rotated, err)
	}
}

func TestManagerBootstrapDoesNotOverwriteAdminConfiguration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	manager, _ := New(store, testSecretMaterial, "https://account.example.test")
	_, _ = manager.Save(ctx, SaveInput{ActorID: "admin-1", Key: "google", ClientID: "admin-id", ClientSecret: "admin-secret", Enabled: false})
	if err := manager.Bootstrap(ctx, BootstrapInput{Key: "google", ClientID: "env-id", ClientSecret: "env-secret", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	view, _ := manager.List(ctx)
	if view[0].Key != "google" || view[0].ClientID != "admin-id" || view[0].Enabled {
		t.Fatalf("bootstrap overwrote admin config: %+v", view)
	}
}
