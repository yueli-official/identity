package oidc_test

import (
	"context"
	"testing"
	"time"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

func TestStoreGetClientFromRepo(t *testing.T) {
	ctx := context.Background()
	r := repo.NewMemory()
	r.SetClient(model.OIDCClient{
		ID: "demo", Public: true,
		RedirectURIs: []string{"http://127.0.0.1/callback"},
		GrantTypes:   []string{"authorization_code"}, ResponseTypes: []string{"code"},
		Scopes: []string{"openid", "profile", "email", "roles"},
	})
	st := oidc.NewStore(r)

	c, err := st.GetClient(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if c.GetID() != "demo" {
		t.Fatalf("id = %q", c.GetID())
	}
	if len(c.GetRedirectURIs()) != 1 || c.GetRedirectURIs()[0] != "http://127.0.0.1/callback" {
		t.Fatalf("redirect uris = %v", c.GetRedirectURIs())
	}

	if _, err := st.GetClient(ctx, "nope"); err == nil {
		t.Fatal("expected error for unknown client")
	}
}

// The provider must accept the Store (proves it satisfies fosite's storage needs).
func TestProviderAcceptsStore(t *testing.T) {
	r := repo.NewMemory()
	kg := func(context.Context) (interface{}, error) { return nil, nil }
	p := oidc.NewProvider(oidc.NewStore(r), oidc.Config{
		Issuer: "http://localhost", GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
		AccessTTL: 10 * time.Minute, IDTTL: 10 * time.Minute,
	}, kg)
	if p == nil {
		t.Fatal("nil provider")
	}
}
