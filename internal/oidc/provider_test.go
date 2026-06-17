package oidc_test

import (
	"context"
	"testing"
	"time"

	"github.com/ory/fosite/handler/oauth2"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/storage"

	"platform/services/identity/internal/oidc"
)

// The session must satisfy BOTH interfaces (the JWT-access-token gotcha).
var _ oauth2.JWTSessionContainer = (*oidc.Session)(nil)
var _ openid.Session = (*oidc.Session)(nil)

func TestNewProviderBuilds(t *testing.T) {
	kg := func(context.Context) (interface{}, error) { return nil, nil }
	p := oidc.NewProvider(storage.NewMemoryStore(), oidc.Config{
		Issuer: "http://localhost", GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
		AccessTTL: 10 * time.Minute, IDTTL: 10 * time.Minute,
	}, kg)
	if p == nil {
		t.Fatal("nil provider")
	}
}

func TestNewSessionShape(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	s := oidc.NewSession("iss", "sub-1", "client-1", "kid-1",
		map[string]interface{}{"email": "a@b.com"},
		map[string]interface{}{"roles": []string{}}, now)
	if s.GetSubject() != "sub-1" {
		t.Fatalf("subject = %q", s.GetSubject())
	}
	if s.GetJWTClaims() == nil || s.GetJWTHeader() == nil {
		t.Fatal("JWT container methods returned nil")
	}
	if s.Clone() == nil {
		t.Fatal("clone nil")
	}
}
