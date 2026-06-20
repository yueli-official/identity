package oidc_test

import (
	"context"
	"testing"

	"platform/gokit/authjwt"
)

// TestAccessTokenVerifiableByGokitAuthjwt is the cross-service contract proof:
// a REAL fosite-signed access token (issued by the live
// IdP stack via authorization_code + PKCE) must pass verification by the shared
// platform/gokit/authjwt verifier — the exact code path the resource services
// (blog / shop / resources) will run when they adopt the JWT middleware.
//
// It closes the gap up front, before another service adopts the middleware:
// it proves authjwt and fosite agree on issuer, the kid header ↔ JWKS lookup,
// RS256, the space-delimited "scope" claim, and the "roles" array — against a
// token minted by the production signing path, not one hand-modeled in a test.
func TestAccessTokenVerifiableByGokitAuthjwt(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Mint a real access token through the full authorize → token flow.
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)

	// Build the verifier exactly as a resource server would: a remote JWKS source
	// pointed at the IdP, and the IdP base URL as the expected issuer.
	v, err := authjwt.NewVerifier(authjwt.VerifierConfig{
		Keys:   authjwt.NewRemoteKeySource(env.base + "/oauth2/jwks.json"),
		Issuer: env.base,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	principal, err := v.Verify(context.Background(), ts.AccessToken)
	if err != nil {
		t.Fatalf("gokit/authjwt rejected a real IdP access token: %v", err)
	}
	if principal.Subject != env.subject {
		t.Fatalf("sub = %q, want %q", principal.Subject, env.subject)
	}
	if principal.Issuer != env.base {
		t.Fatalf("iss = %q, want %q", principal.Issuer, env.base)
	}
	if !principal.HasScope("openid") || !principal.HasScope("roles") {
		t.Fatalf("scopes = %v, want to include openid + roles", principal.Scopes)
	}
	// The freshly registered user is granted the default "user" role (⑥-RBAC),
	// which the access token carries because the "roles" scope was granted.
	if !principal.HasRole("user") {
		t.Fatalf("roles = %v, want to include default 'user'", principal.Roles)
	}
	t.Logf("cross-contract OK: real fosite token verified by gokit/authjwt — sub=%s scopes=%v roles=%v",
		principal.Subject, principal.Scopes, principal.Roles)
}
