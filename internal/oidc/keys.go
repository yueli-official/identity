// Package oidc wires ory/fosite into the identity service: signing keys,
// provider, session, storage adapter, and scope→claim mapping.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// Manager holds the in-memory active private key (loaded once at startup) and
// the public JWKS (active + retired), loaded once so we don't hit the DB per request.
type Manager struct {
	keys      repo.SigningKeyRepo
	activeKID string
	activeKey *rsa.PrivateKey
	jwks      jose.JSONWebKeySet
}

// NewManager bootstraps (generates one active RS256 key if none exists) then
// loads the active private key and builds the JWKS into memory.
func NewManager(ctx context.Context, keys repo.SigningKeyRepo) (*Manager, error) {
	m := &Manager{keys: keys}
	if err := m.bootstrap(ctx); err != nil {
		return nil, err
	}
	if err := m.load(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) bootstrap(ctx context.Context) error {
	if _, err := m.keys.GetActiveKey(ctx); err == nil {
		return nil
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	kid := uuid.NewString()
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return m.keys.InsertKey(ctx, model.SigningKey{
		KID: kid, Alg: "RS256", PrivatePEM: string(privPEM), PublicPEM: string(pubPEM),
		Status: model.KeyActive,
	})
}

func (m *Manager) load(ctx context.Context) error {
	active, err := m.keys.GetActiveKey(ctx)
	if err != nil {
		return err
	}
	priv, err := parseRSAPrivate(active.PrivatePEM)
	if err != nil {
		return fmt.Errorf("parse active key: %w", err)
	}
	m.activeKID = active.KID
	m.activeKey = priv

	all, err := m.keys.ListPublicKeys(ctx)
	if err != nil {
		return err
	}
	var set jose.JSONWebKeySet
	for _, k := range all {
		pub, err := parseRSAPublic(k.PublicPEM)
		if err != nil {
			return fmt.Errorf("parse public key %s: %w", k.KID, err)
		}
		set.Keys = append(set.Keys, jose.JSONWebKey{Key: pub, KeyID: k.KID, Algorithm: "RS256", Use: "sig"})
	}
	m.jwks = set
	return nil
}

// ActiveKID returns the key ID of the current active signing key.
func (m *Manager) ActiveKID() string { return m.activeKID }

// ActivePrivateKey returns the RSA private key used to sign tokens.
func (m *Manager) ActivePrivateKey() *rsa.PrivateKey { return m.activeKey }

// JWKS returns the public JSON Web Key Set (active + retired keys).
func (m *Manager) JWKS() jose.JSONWebKeySet { return m.jwks }

// PublicKey lets the identity service validate its own scoped access tokens
// without making an HTTP request back through its JWKS endpoint.
func (m *Manager) PublicKey(_ context.Context, kid string) (any, error) {
	keys := m.jwks.Key(kid)
	if len(keys) != 1 || keys[0].Key == nil {
		return nil, fmt.Errorf("signing key %q not found", kid)
	}
	return keys[0].Key, nil
}

// KeyGetter is fosite's key getter: returns the active private key.
func (m *Manager) KeyGetter(context.Context) (interface{}, error) { return m.activeKey, nil }

// MintServiceToken self-signs a short-lived RS256 access token (kid in JWKS) for
// the given subject. Used for first-party server-to-server calls where the IdP
// acts on behalf of a logged-in user (e.g. proxying an avatar upload to the
// asset service): the user authenticates to the IdP by session cookie, and the
// IdP mints a user-scoped bearer the resource server verifies via JWKS. audience
// is mandatory and identifies that resource server; scope is space-delimited
// and may be empty.
func (m *Manager) MintServiceToken(issuer, subject, audience, scope string, ttl time.Duration, now time.Time) (string, error) {
	if strings.TrimSpace(audience) == "" {
		return "", fmt.Errorf("service token audience is required")
	}
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.activeKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", m.activeKID),
	)
	if err != nil {
		return "", err
	}
	claims := jwt.Claims{
		Issuer:    issuer,
		Subject:   subject,
		Audience:  jwt.Audience{strings.TrimSpace(audience)},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(ttl)),
	}
	builder := jwt.Signed(sig).Claims(claims)
	extra := map[string]interface{}{"client_id": "identity-svc"}
	if scope != "" {
		extra["scope"] = scope
	}
	builder = builder.Claims(extra)
	return builder.CompactSerialize()
}

func (m *Manager) MintGuestToken(issuer, subject, clientID, audience string, ttl time.Duration, now time.Time) (string, error) {
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(clientID) == "" || strings.TrimSpace(audience) == "" {
		return "", fmt.Errorf("guest token subject, client id and audience are required")
	}
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: m.activeKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", m.activeKID),
	)
	if err != nil {
		return "", err
	}
	claims := jwt.Claims{
		Issuer: issuer, Subject: subject, Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(ttl)),
		ID: uuid.NewString(),
	}
	return jwt.Signed(sig).Claims(claims).Claims(map[string]interface{}{
		"client_id": clientID, "subject_kind": "guest", "scope": "guest:access",
	}).CompactSerialize()
}

func parseRSAPrivate(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func parseRSAPublic(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}
