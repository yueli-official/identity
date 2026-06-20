package oidc

import (
	"context"
	"testing"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/oauth2"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/handler/pkce"
	"github.com/ory/fosite/storage"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// Compile-time: Store satisfies every fosite storage facet the provider uses.
var (
	_ fosite.Storage                     = (*Store)(nil)
	_ oauth2.CoreStorage                 = (*Store)(nil)
	_ oauth2.TokenRevocationStorage      = (*Store)(nil)
	_ pkce.PKCERequestStorage            = (*Store)(nil)
	_ openid.OpenIDConnectRequestStorage = (*Store)(nil)
	_ storage.Transactional              = (*Store)(nil)
)

func TestStoreAuthCodeRoundTripAndInvalidate(t *testing.T) {
	ctx := context.Background()
	r := repo.NewMemory()
	r.SetClient(model.OIDCClient{ID: "demo", Public: true, Scopes: []string{"openid"}})
	st := NewStore(newMemBackend(), r)

	req := &fosite.Request{
		ID: "req-1", RequestedAt: time.Now().UTC(),
		Client:  &fosite.DefaultClient{ID: "demo"},
		Session: NewSession("iss", "sub-1", "demo", "kid", nil, nil, time.Now().UTC()),
	}
	if err := st.CreateAuthorizeCodeSession(ctx, "code-sig", req); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAuthorizeCodeSession(ctx, "code-sig", NewSession("", "", "", "", nil, nil, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if got.GetClient().GetID() != "demo" {
		t.Fatalf("client lost: %q", got.GetClient().GetID())
	}
	if err := st.InvalidateAuthorizeCodeSession(ctx, "code-sig"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetAuthorizeCodeSession(ctx, "code-sig", got.GetSession()); err != fosite.ErrInvalidatedAuthorizeCode {
		t.Fatalf("want ErrInvalidatedAuthorizeCode, got %v", err)
	}
}

func TestStoreRefreshInactiveMapsToFositeError(t *testing.T) {
	ctx := context.Background()
	r := repo.NewMemory()
	r.SetClient(model.OIDCClient{ID: "demo", Public: true})
	st := NewStore(newMemBackend(), r)
	req := &fosite.Request{
		ID: "req-1", Client: &fosite.DefaultClient{ID: "demo"},
		Session: NewSession("iss", "sub-1", "demo", "kid", nil, nil, time.Now().UTC()),
	}
	req.Session.(*Session).IdPSessionID = "sess-1"
	if err := st.CreateRefreshTokenSession(ctx, "rt-sig", "at-sig", req); err != nil {
		t.Fatal(err)
	}
	if err := st.RotateRefreshToken(ctx, "req-1", "rt-sig"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetRefreshTokenSession(ctx, "rt-sig", req.Session); err != fosite.ErrInactiveToken {
		t.Fatalf("want ErrInactiveToken, got %v", err)
	}
}
