package oidc_test

import (
	"context"
	"crypto/rsa"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"

	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

func TestMintServiceTokenIdentifiesIdentityService(t *testing.T) {
	ctx := context.Background()
	m, err := oidc.NewManager(ctx, repo.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := m.MintServiceToken("https://identity.test", "user-1", "", time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := josejwt.ParseSigned(raw)
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		ClientID string `json:"client_id"`
	}
	if err := parsed.Claims(&m.ActivePrivateKey().PublicKey, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.ClientID != "identity-svc" {
		t.Fatalf("client_id = %q, want identity-svc", claims.ClientID)
	}
}

func TestBootstrapGeneratesActiveKeyWhenEmpty(t *testing.T) {
	ctx := context.Background()
	r := repo.NewMemory()
	m, err := oidc.NewManager(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if m.ActiveKID() == "" {
		t.Fatal("no active kid after bootstrap")
	}
	if m.ActivePrivateKey() == nil {
		t.Fatal("no active private key")
	}
	jwks := m.JWKS()
	if len(jwks.Keys) != 1 {
		t.Fatalf("want 1 jwks key, got %d", len(jwks.Keys))
	}
	if jwks.Keys[0].KeyID != m.ActiveKID() {
		t.Fatalf("jwks kid mismatch: %s vs %s", jwks.Keys[0].KeyID, m.ActiveKID())
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r := repo.NewMemory()
	m1, err := oidc.NewManager(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := oidc.NewManager(ctx, r) // same repo: must reuse the existing active key
	if err != nil {
		t.Fatal(err)
	}
	if m1.ActiveKID() != m2.ActiveKID() {
		t.Fatalf("bootstrap not idempotent: %s vs %s", m1.ActiveKID(), m2.ActiveKID())
	}
}

func TestJWKSPublicKeyVerifiesSignature(t *testing.T) {
	ctx := context.Background()
	m, err := oidc.NewManager(ctx, repo.NewMemory())
	if err != nil {
		t.Fatal(err)
	}

	// 1. Verify the JWKS contains exactly one key whose .Key is an *rsa.PublicKey
	//    that matches the public component of the active private key.
	kid := m.ActiveKID()
	jwks := m.JWKS()
	matched := jwks.Key(kid)
	if len(matched) != 1 {
		t.Fatalf("JWKS.Key(%q) returned %d keys, want 1", kid, len(matched))
	}
	jwkPub, ok := matched[0].Key.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("JWKS key is not *rsa.PublicKey, got %T", matched[0].Key)
	}
	activePub := &m.ActivePrivateKey().PublicKey
	if jwkPub.N.Cmp(activePub.N) != 0 || jwkPub.E != activePub.E {
		t.Fatal("JWKS public key does not match active private key's public component")
	}

	// 2. Sign a minimal JWT with the active private key and verify it using the
	//    JWKS public key — proves the two sides are consistent end-to-end.
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.ActivePrivateKey()},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	type claims struct {
		Sub string `json:"sub"`
		Iss string `json:"iss"`
	}
	raw, err := josejwt.Signed(sig).Claims(claims{Sub: "test-user", Iss: "test-issuer"}).CompactSerialize()
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	parsed, err := josejwt.ParseSigned(raw)
	if err != nil {
		t.Fatalf("parse signed JWT: %v", err)
	}
	if parsed.Headers[0].Algorithm != string(jose.RS256) {
		t.Fatalf("token alg = %q, want RS256", parsed.Headers[0].Algorithm)
	}

	var got claims
	if err := parsed.Claims(jwkPub, &got); err != nil {
		t.Fatalf("RS256 signature verification FAILED: %v", err)
	}
	if got.Sub != "test-user" {
		t.Fatalf("sub claim = %q, want test-user", got.Sub)
	}
}
