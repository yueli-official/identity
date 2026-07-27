package oidc_test

import (
	"context"
	"testing"

	"github.com/yueli-official/foundation/go/jwks"
	identityruntime "platform/services/identity/internal/runtime"
)

// TestAccessTokenVerifiableByFoundation is the cross-service contract proof:
// a REAL fosite-signed access token (issued by the live
// IdP stack via authorization_code + PKCE) must pass verification by the public
// Foundation verifier through Platform's deployment-config adapter.
//
// It closes the gap up front, before another service adopts the middleware:
// it proves Foundation auth and fosite agree on issuer, the kid header ↔ JWKS lookup,
// RS256, the space-delimited "scope" claim, and the "roles" array — against a
// token minted by the production signing path, not one hand-modeled in a test.
func TestAccessTokenVerifiableByFoundation(t *testing.T) {
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
	v, err := identityruntime.NewRemoteVerifier(identityruntime.RemoteVerifierConfig{
		JWKSURL: env.base + "/oauth2/jwks.json",
		Issuer:  env.base,
		Transport: jwks.RemoteOptions{
			AllowLoopbackHTTP: true,
		},
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	principal, err := v.Verify(context.Background(), ts.AccessToken)
	if err != nil {
		t.Fatalf("Foundation rejected a real IdP access token: %v", err)
	}
	if principal.Subject != env.subject {
		t.Fatalf("sub = %q, want %q", principal.Subject, env.subject)
	}
	if principal.Issuer != env.base {
		t.Fatalf("iss = %q, want %q", principal.Issuer, env.base)
	}
	if principal.ClientID != clientID {
		t.Fatalf("client_id = %q, want %q", principal.ClientID, clientID)
	}
	if !principal.HasScope("openid") || !principal.HasScope("roles") {
		t.Fatalf("scopes = %v, want to include openid + roles", principal.Scopes)
	}
	// The freshly registered user is granted the default "user" role (⑥-RBAC),
	// which the access token carries because the "roles" scope was granted.
	if !principal.HasRole("user") {
		t.Fatalf("roles = %v, want to include default 'user'", principal.Roles)
	}
	t.Logf("cross-contract OK: real fosite token verified by Foundation — sub=%s scopes=%v roles=%v",
		principal.Subject, principal.Scopes, principal.Roles)
}
