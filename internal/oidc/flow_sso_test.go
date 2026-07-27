package oidc_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/yueli-official/identity/internal/model"
)

// TestOIDCMultiClientSSO proves single sign-on across two OIDC clients: one IdP
// login session lets BOTH clients complete authorization_code WITHOUT a second
// login, and after logout neither can (the /authorize redirects to login). This
// is the multi-client SSO + logout-chain acceptance test; the
// single-client SSO / logout / passive-logout paths are covered by flow_test.go.
func TestOIDCMultiClientSSO(t *testing.T) {
	env := setupE2E(t, "client-a")

	// A second consumer-site client sharing the same IdP.
	env.repo.SetClient(model.OIDCClient{
		ID:            "client-b",
		Public:        true,
		RedirectURIs:  []string{callbackURI},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile", "email", "roles"},
	})
	const scope = "openid profile email roles"

	// Client A — authorize + token using the login session.
	pA := newPKCE(t)
	codeA := authorizeForCode(t, env, "client-a", scope, pA)
	sA, bA, tA := exchangeCode(t, env, "client-a", codeA, pA.verifier)
	assertValidTokenResponse(t, sA, bA, tA, true)
	verifyAccessTokenLocally(t, env.base, tA.AccessToken, env.base, env.subject)

	// Client B — SAME id_session cookie, NO second login. authorizeForCode fails
	// the test if /authorize redirects to login instead of issuing a code, so a
	// returned code IS the SSO proof.
	pB := newPKCE(t)
	codeB := authorizeForCode(t, env, "client-b", scope, pB)
	sB, bB, tB := exchangeCode(t, env, "client-b", codeB, pB.verifier)
	assertValidTokenResponse(t, sB, bB, tB, true)
	verifyAccessTokenLocally(t, env.base, tB.AccessToken, env.base, env.subject)
	t.Logf("SSO OK: one login → client-a + client-b both issued tokens (sub=%s)", env.subject)

	// Logout breaks SSO: clearing the IdP session must make /authorize redirect to
	// login for any client.
	if err := env.svc.Logout(env.ctx, env.sid); err != nil {
		t.Fatalf("logout: %v", err)
	}
	pC := newPKCE(t)
	authURL := env.base + "/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"client-b"},
		"redirect_uri":          {callbackURI},
		"scope":                 {scope},
		"state":                 {"post-logout"},
		"code_challenge":        {pC.challenge},
		"code_challenge_method": {"S256"},
	}.Encode()
	req, err := http.NewRequestWithContext(env.ctx, http.MethodGet, authURL, nil)
	if err != nil {
		t.Fatalf("build post-logout authorize: %v", err)
	}
	req.Header.Set("Cookie", "id_session="+env.sid)
	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatalf("post-logout authorize: %v", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusFound || !strings.HasPrefix(loc, env.base+"/login") {
		t.Fatalf("after logout, authorize must redirect to login; got %d %q", resp.StatusCode, loc)
	}
	t.Logf("logout broke SSO: authorize → %s", loc)
}
