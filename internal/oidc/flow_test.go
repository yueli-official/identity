package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gogf/gf/v2/frame/g"
	"golang.org/x/crypto/bcrypt"

	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

// callbackURI is the redirect_uri shared by the in-memory demo clients seeded by
// the hermetic e2e tests below.
const callbackURI = "http://127.0.0.1/callback"

// noRedirectClient returns an HTTP client that does NOT follow redirects, so the
// /authorize 302 Location (carrying the authorization code) can be read.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// e2eEnv holds the wired-up hermetic OIDC stack: the in-memory repo, the logic
// service (with the OIDC store wired as refresh revoker), the live g.Server base
// URL, and the login session identifiers for the seeded user.
type e2eEnv struct {
	ctx     context.Context
	repo    *repo.Memory
	svc     *logic.Service
	store   *oidc.Store
	base    string
	sid     string // id_session cookie value
	subject string // identity ID == JWT sub
}

// setupE2E builds the full hermetic OIDC stack used by the e2e tests:
//   - in-memory repo + a demo client (authorization_code + refresh_token grants,
//     offline_access scope so refresh can be exercised),
//   - a registered+logged-in user (yielding the id_session cookie value),
//   - manager → store → provider → controller, with svc.SetRefreshRevoker(store)
//     wired (mirrors main.go) so /end_session performs passive logout,
//   - a g.Server mounting ALL OIDC routes (incl. /revoke + /end_session).
//
// The server is registered for automatic shutdown via t.Cleanup.
func setupE2E(t *testing.T, clientID string) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	// 1. In-memory repo + demo client. Unlike ③'s minimal client, this one also
	//    advertises the refresh_token grant and offline_access scope.
	r := repo.NewMemory()
	r.SetClient(model.OIDCClient{
		ID:                     clientID,
		Public:                 true,
		RedirectURIs:           []string{callbackURI},
		PostLogoutRedirectURIs: []string{"http://127.0.0.1/after-logout"},
		Audiences:              []string{clientID, "api://" + clientID},
		GrantTypes:             []string{"authorization_code", "refresh_token"},
		ResponseTypes:          []string{"code"},
		Scopes:                 []string{"openid", "profile", "email", "roles", "offline_access"},
	})

	// 2. User: register + login → session id + subject.
	svc := logic.New(r, logic.DefaultConfig())
	if _, err := svc.Register(ctx, logic.RegisterInput{
		Email: "u@e.com", Password: "correct horse battery", DisplayName: "U",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	loginOut, err := svc.Login(ctx, logic.LoginInput{
		Email: "u@e.com", Password: "correct horse battery", IP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	sid := loginOut.SessionID
	subject := loginOut.Identity.ID

	// 3. Pick a free port so the issuer URL exists before the server starts.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close temp listener: %v", err)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 4. Build OIDC stack (manager → store → provider → controller). A non-zero
	//    RefreshTTL is required so issued refresh tokens have a valid lifespan.
	mgr, err := oidc.NewManager(ctx, r)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	store := oidc.NewStore(oidc.NewMemBackend(), r)
	// CRITICAL wiring: without this, svc.Logout (called by EndSession) cannot
	// revoke session-bound refresh tokens and the passive-logout test would not
	// actually verify anything. Mirrors main.go.
	svc.SetRefreshRevoker(store)
	provider := oidc.NewProvider(store, oidc.Config{
		Issuer:       base,
		GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
		RefreshTTL:   720 * time.Hour,
	}, mgr.KeyGetter)
	ctl := controller.NewOIDC(
		provider, mgr, svc, r, base, base+"/login", false,
		[]byte("0123456789abcdef0123456789abcdef"),
	)

	// 5. Start GoFrame server on the pre-chosen port (NO ghttpx.Middleware), with
	//    the full OIDC route set.
	s := g.Server(t.Name())
	s.SetAddr(fmt.Sprintf("127.0.0.1:%d", port)) // loopback-only: avoids the Windows Firewall prompt
	s.SetDumpRouterMap(false)
	s.BindHandler("GET:/.well-known/openid-configuration", ctl.Discovery)
	s.BindHandler("GET:/oauth2/jwks.json", ctl.JWKS)
	s.BindHandler("GET:/oauth2/authorize", ctl.Authorize)
	s.BindHandler("POST:/oauth2/token", ctl.Token)
	s.BindHandler("ALL:/oauth2/userinfo", ctl.Userinfo)
	s.BindHandler("POST:/oauth2/revoke", ctl.Revoke)
	s.BindHandler("ALL:/oauth2/end_session", ctl.EndSession)
	s.Start()
	t.Cleanup(func() { _ = s.Shutdown() })

	return &e2eEnv{
		ctx:     ctx,
		repo:    r,
		svc:     svc,
		store:   store,
		base:    base,
		sid:     sid,
		subject: subject,
	}
}

// tokenSet is the parsed /oauth2/token JSON response.
type tokenSet struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// pkcePair is a PKCE code_verifier + its S256 challenge.
type pkcePair struct {
	verifier  string
	challenge string
}

// newPKCE generates a PKCE verifier + S256 challenge (mirrors the PoC).
func newPKCE(t *testing.T) pkcePair {
	t.Helper()
	verifier := randPKCEVerifier(t)
	sum := sha256.Sum256([]byte(verifier))
	return pkcePair{
		verifier:  verifier,
		challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
	}
}

// authorizeForCode drives GET /oauth2/authorize with the given cookie and PKCE
// challenge and returns the authorization code from the 302 Location. It fails
// the test if the authorize step does not produce a code at the callback URI.
func authorizeForCode(t *testing.T, env *e2eEnv, clientID, scope string, p pkcePair) string {
	t.Helper()
	authURL := env.base + "/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {callbackURI},
		"scope":                 {scope},
		"state":                 {"xyz-test-state-123"},
		"code_challenge":        {p.challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	req, err := http.NewRequestWithContext(env.ctx, http.MethodGet, authURL, nil)
	if err != nil {
		t.Fatalf("build authorize request: %v", err)
	}
	req.Header.Set("Cookie", "id_session="+env.sid)

	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatalf("authorize request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("authorize: expected 302/303, got %d: %s", resp.StatusCode, body)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize redirect Location: %v", err)
	}
	if !strings.HasPrefix(loc.String(), callbackURI) {
		t.Fatalf("authorize: location does not start with callback URI, got %q", loc.String())
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("authorize: no authorization code in redirect: %s", loc.String())
	}
	return code
}

// exchangeCode POSTs /oauth2/token with the authorization_code grant and returns
// the raw HTTP status, the response body, and the parsed token set.
func exchangeCode(t *testing.T, env *e2eEnv, clientID, code, verifier string) (int, []byte, tokenSet) {
	t.Helper()
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {callbackURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	return postToken(t, env, form)
}

// refreshToken POSTs /oauth2/token with the refresh_token grant (public client;
// no PKCE on refresh) and returns the raw HTTP status, body, and parsed set.
func refreshToken(t *testing.T, env *e2eEnv, clientID, rt string) (int, []byte, tokenSet) {
	t.Helper()
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt},
		"client_id":     {clientID},
	}
	return postToken(t, env, form)
}

// postToken POSTs a form to /oauth2/token and parses the JSON body.
func postToken(t *testing.T, env *e2eEnv, form url.Values) (int, []byte, tokenSet) {
	t.Helper()
	req, err := http.NewRequestWithContext(env.ctx, http.MethodPost,
		env.base+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var ts tokenSet
	_ = json.Unmarshal(body, &ts) // non-200 bodies are error JSON; tolerated
	return resp.StatusCode, body, ts
}

// assertValidTokenResponse asserts a 200 token response with a JWT access token
// and bearer type. id_token presence is asserted only when requireIDToken is set
// (true for the authorization_code exchange; the refresh_token grant does not
// re-mint an id_token in this configuration). It does NOT assert refresh
// presence (callers that requested offline_access check that separately).
func assertValidTokenResponse(t *testing.T, status int, body []byte, ts tokenSet, requireIDToken bool) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("token: expected 200, got %d: %s", status, body)
	}
	if ts.AccessToken == "" {
		t.Fatalf("token: empty access_token in: %s", body)
	}
	if requireIDToken && ts.IDToken == "" {
		t.Fatalf("token: empty id_token in: %s", body)
	}
	if !strings.EqualFold(ts.TokenType, "bearer") {
		t.Fatalf("token: token_type = %q, want bearer", ts.TokenType)
	}
	if n := strings.Count(ts.AccessToken, "."); n != 2 {
		t.Fatalf("token: access_token not a JWT (want 2 dots, got %d)", n)
	}
}

// TestOIDCClientCredentialsServiceToken is the resource-site acceptance test
// for the service-token chain's first link: a confidential client mints a
// client_credentials access token carrying its requested scope, so a sibling
// service (e.g. the resource site) can authorize on it (asset:sign). It guards a
// fosite v0.49 sharp edge — ClientCredentialsGrantHandler VALIDATES the
// requested scopes against the client's allowlist but does NOT grant them, so
// the controller must GrantScope itself (applyServiceClaims); otherwise the
// token ships an empty "scope" claim and every downstream scope check fails.
func TestOIDCClientCredentialsServiceToken(t *testing.T) {
	env := setupE2E(t, "demo-cc")

	// Seed a confidential service client: bcrypt secret, client_credentials only,
	// allowed scope asset:sign.
	const secret = "svc-secret-0123456789"
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash secret: %v", err)
	}
	env.repo.SetClient(model.OIDCClient{
		ID:         "svc-test",
		Public:     false,
		SecretHash: string(hash),
		GrantTypes: []string{"client_credentials"},
		Scopes:     []string{"asset:sign"},
	})

	// Happy path: client_secret_post → 200, sub = client, scope granted, kid set.
	status, body, ts := postToken(t, env, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"svc-test"},
		"client_secret": {secret},
		"scope":         {"asset:sign"},
	})
	if status != http.StatusOK {
		t.Fatalf("cc grant: want 200, got %d: %s", status, body)
	}
	if n := strings.Count(ts.AccessToken, "."); n != 2 {
		t.Fatalf("cc grant: access_token not a JWT: %s", body)
	}
	sub, scope, kid := decodeJWTClaims(t, ts.AccessToken)
	if sub != "svc-test" {
		t.Fatalf("cc grant: sub = %q, want svc-test", sub)
	}
	if scope != "asset:sign" {
		t.Fatalf("cc grant: scope = %q, want asset:sign (fosite does not GrantScope itself)", scope)
	}
	if kid == "" {
		t.Fatalf("cc grant: missing kid header → resource servers cannot resolve the signing key")
	}

	// Wrong secret → invalid_client, no token (confidential auth enforced).
	badStatus, badBody, badTs := postToken(t, env, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"svc-test"},
		"client_secret": {"WRONG"},
		"scope":         {"asset:sign"},
	})
	if badStatus == http.StatusOK || badTs.AccessToken != "" {
		t.Fatalf("cc grant with bad secret: expected failure, got %d: %s", badStatus, badBody)
	}
	if !strings.Contains(string(badBody), "invalid_client") {
		t.Fatalf("cc grant with bad secret: want invalid_client, got: %s", badBody)
	}
}

// decodeJWTClaims splits a compact JWT and returns its sub + scope claims and the
// kid header (no signature check — claim presence is what this test asserts).
func decodeJWTClaims(t *testing.T, token string) (sub, scope, kid string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("decodeJWTClaims: not a compact JWT")
	}
	var hdr struct {
		Kid string `json:"kid"`
	}
	var claims struct {
		Sub   string `json:"sub"`
		Scope string `json:"scope"`
	}
	for _, d := range []struct {
		seg string
		dst any
	}{{parts[0], &hdr}, {parts[1], &claims}} {
		raw, err := base64.RawURLEncoding.DecodeString(d.seg)
		if err != nil {
			t.Fatalf("decodeJWTClaims: base64: %v", err)
		}
		if err := json.Unmarshal(raw, d.dst); err != nil {
			t.Fatalf("decodeJWTClaims: json: %v", err)
		}
	}
	return claims.Sub, claims.Scope, hdr.Kid
}

// TestOIDCFlow is the OIDC acceptance test: hermetic end-to-end
// authorization_code + PKCE flow, with local JWKS verification of the JWT
// access token (mirrors the PoC's verifyAccessTokenLocally).
func TestOIDCFlow(t *testing.T) {
	ctx := context.Background()

	// -----------------------------------------------------------------------
	// 1. In-memory repo + demo client
	// -----------------------------------------------------------------------
	r := repo.NewMemory()
	r.SetClient(model.OIDCClient{
		ID:            "demo",
		Public:        true,
		RedirectURIs:  []string{"http://127.0.0.1/callback"},
		GrantTypes:    []string{"authorization_code"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile", "email", "roles"},
	})

	// -----------------------------------------------------------------------
	// 2. User: register + login → session id + subject
	// -----------------------------------------------------------------------
	svc := logic.New(r, logic.DefaultConfig())
	if _, err := svc.Register(ctx, logic.RegisterInput{
		Email: "u@e.com", Password: "correct horse battery", DisplayName: "U",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	loginOut, err := svc.Login(ctx, logic.LoginInput{
		Email: "u@e.com", Password: "correct horse battery", IP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	sid := loginOut.SessionID
	subject := loginOut.Identity.ID
	t.Logf("logged in: sid=%s subject=%s", sid, subject)

	// -----------------------------------------------------------------------
	// 3. Pick a free port so we can build the issuer URL before starting
	//    the server (provider/controller need the issuer at construction time).
	// -----------------------------------------------------------------------
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close temp listener: %v", err)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	t.Logf("issuer / base URL = %s", base)

	// -----------------------------------------------------------------------
	// 4. Build OIDC stack (manager → store → provider → controller)
	// -----------------------------------------------------------------------
	mgr, err := oidc.NewManager(ctx, r)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	store := oidc.NewStore(oidc.NewMemBackend(), r)
	provider := oidc.NewProvider(store, oidc.Config{
		Issuer:       base,
		GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
	}, mgr.KeyGetter)
	ctl := controller.NewOIDC(
		provider, mgr, svc, r, base, base+"/login", false,
		[]byte("0123456789abcdef0123456789abcdef"),
	)

	// -----------------------------------------------------------------------
	// 5. Start GoFrame server on the pre-chosen port (NO ghttpx.Middleware)
	// -----------------------------------------------------------------------
	s := g.Server(t.Name())
	s.SetAddr(fmt.Sprintf("127.0.0.1:%d", port)) // loopback-only: avoids the Windows Firewall prompt
	s.SetDumpRouterMap(false)
	// Bind OIDC handlers directly — no middleware wrapper so fosite can write
	// RFC-compliant headers + bodies through RawWriter without interference.
	s.BindHandler("GET:/.well-known/openid-configuration", ctl.Discovery)
	s.BindHandler("GET:/oauth2/jwks.json", ctl.JWKS)
	s.BindHandler("GET:/oauth2/authorize", ctl.Authorize)
	s.BindHandler("POST:/oauth2/token", ctl.Token)
	s.BindHandler("ALL:/oauth2/userinfo", ctl.Userinfo)
	s.Start()
	defer s.Shutdown()

	// -----------------------------------------------------------------------
	// 6. PKCE: verifier + S256 challenge (mirrors the PoC)
	// -----------------------------------------------------------------------
	verifier := randPKCEVerifier(t)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	// HTTP client that does NOT follow redirects so we can read Location.
	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// -----------------------------------------------------------------------
	// 7. Authorize — happy path (with session cookie)
	// -----------------------------------------------------------------------
	authURL := base + "/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {"demo"},
		"redirect_uri":          {"http://127.0.0.1/callback"},
		"scope":                 {"openid profile email roles"},
		"state":                 {"xyz-test-state-123"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	authReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		t.Fatalf("build authorize request: %v", err)
	}
	authReq.Header.Set("Cookie", "id_session="+sid)

	authResp, err := noRedirect.Do(authReq)
	if err != nil {
		t.Fatalf("authorize request: %v", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusFound && authResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(authResp.Body)
		t.Fatalf("authorize: expected 302/303, got %d: %s", authResp.StatusCode, body)
	}
	loc, err := url.Parse(authResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize redirect Location: %v", err)
	}
	t.Logf("authorize redirect → %s", loc.String())
	if !strings.HasPrefix(loc.String(), "http://127.0.0.1/callback") {
		t.Fatalf("authorize: location does not start with callback URI, got %q", loc.String())
	}
	if got := loc.Query().Get("state"); got != "xyz-test-state-123" {
		t.Fatalf("authorize: state mismatch, got %q want %q", got, "xyz-test-state-123")
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("authorize: no authorization code in redirect: %s", loc.String())
	}
	t.Logf("authorize OK: code extracted, state=xyz confirmed")

	// -----------------------------------------------------------------------
	// 8. Token — exchange code for tokens
	// -----------------------------------------------------------------------
	tokForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://127.0.0.1/callback"},
		"client_id":     {"demo"},
		"code_verifier": {verifier},
	}
	tokReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/oauth2/token", strings.NewReader(tokForm.Encode()))
	if err != nil {
		t.Fatalf("build token request: %v", err)
	}
	tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokResp, err := http.DefaultClient.Do(tokReq)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer tokResp.Body.Close()
	tokBody, _ := io.ReadAll(tokResp.Body)
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token: expected 200, got %d: %s", tokResp.StatusCode, tokBody)
	}

	var tokenSet struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(tokBody, &tokenSet); err != nil {
		t.Fatalf("decode token response: %v (%s)", err, tokBody)
	}
	if tokenSet.AccessToken == "" {
		t.Fatalf("token: empty access_token in: %s", tokBody)
	}
	if tokenSet.IDToken == "" {
		t.Fatalf("token: empty id_token in: %s", tokBody)
	}
	if !strings.EqualFold(tokenSet.TokenType, "bearer") {
		t.Fatalf("token: token_type = %q, want bearer", tokenSet.TokenType)
	}
	// Must be a JWT (3 dot-separated parts).
	if n := strings.Count(tokenSet.AccessToken, "."); n != 2 {
		t.Fatalf("token: access_token not a JWT (want 2 dots, got %d)", n)
	}
	t.Logf("token OK: access_token JWT len=%d, id_token len=%d",
		len(tokenSet.AccessToken), len(tokenSet.IDToken))

	// -----------------------------------------------------------------------
	// 9. Local JWKS verification of the access token (the core proof)
	// -----------------------------------------------------------------------
	verifyAccessTokenLocally(t, base, tokenSet.AccessToken, base, subject)

	// -----------------------------------------------------------------------
	// 10. Userinfo
	// -----------------------------------------------------------------------
	uiReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/oauth2/userinfo", nil)
	uiReq.Header.Set("Authorization", "Bearer "+tokenSet.AccessToken)
	uiResp, err := http.DefaultClient.Do(uiReq)
	if err != nil {
		t.Fatalf("userinfo request: %v", err)
	}
	defer uiResp.Body.Close()
	uiBody, _ := io.ReadAll(uiResp.Body)
	if uiResp.StatusCode != http.StatusOK {
		t.Fatalf("userinfo: expected 200, got %d: %s", uiResp.StatusCode, uiBody)
	}
	var userinfo map[string]interface{}
	if err := json.Unmarshal(uiBody, &userinfo); err != nil {
		t.Fatalf("decode userinfo: %v (%s)", err, uiBody)
	}
	if userinfo["sub"] != subject {
		t.Fatalf("userinfo: sub = %v, want %s", userinfo["sub"], subject)
	}
	if userinfo["email"] != "u@e.com" {
		t.Fatalf("userinfo: email = %v, want u@e.com", userinfo["email"])
	}
	if _, ok := userinfo["roles"]; !ok {
		t.Fatalf("userinfo: 'roles' key missing from response: %v", userinfo)
	}
	t.Logf("userinfo OK: sub=%v email=%v roles=%v",
		userinfo["sub"], userinfo["email"], userinfo["roles"])

	// -----------------------------------------------------------------------
	// 11. Negative: authorize WITHOUT session cookie → redirect to /login
	// -----------------------------------------------------------------------
	noSessionReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		t.Fatalf("build no-session authorize request: %v", err)
	}
	// Deliberately no Cookie header.
	noSessionResp, err := noRedirect.Do(noSessionReq)
	if err != nil {
		t.Fatalf("no-session authorize request: %v", err)
	}
	defer noSessionResp.Body.Close()

	if noSessionResp.StatusCode != http.StatusFound && noSessionResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(noSessionResp.Body)
		t.Fatalf("no-session: expected redirect, got %d: %s", noSessionResp.StatusCode, body)
	}
	noSessLoc := noSessionResp.Header.Get("Location")
	if !strings.HasPrefix(noSessLoc, base+"/login") {
		t.Fatalf("no-session: expected redirect to %s/login..., got %q", base, noSessLoc)
	}
	t.Logf("no-session OK: correctly redirected to %s", noSessLoc)
}

// TestOIDCRefreshFlow is the refresh happy path: an authorize with
// offline_access yields a refresh token; the refresh_token grant then mints a
// NEW access token (JWKS-verified) and a NEW, distinct refresh token (rotation).
func TestOIDCRefreshFlow(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Authorize → token (with offline_access → refresh token expected).
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)
	if ts.RefreshToken == "" {
		t.Fatalf("offline_access requested but no refresh_token in response: %s", body)
	}
	verifyAccessTokenLocally(t, env.base, ts.AccessToken, env.base, env.subject)
	assertAccessTokenAudiences(t, ts.AccessToken, []string{clientID, "api://" + clientID})
	t.Logf("initial token OK: refresh_token len=%d", len(ts.RefreshToken))

	// Refresh grant → NEW access token + NEW refresh token (rotation). The refresh
	// grant does not re-mint an id_token, so do not require one here.
	rStatus, rBody, rts := refreshToken(t, env, clientID, ts.RefreshToken)
	assertValidTokenResponse(t, rStatus, rBody, rts, false)
	if rts.RefreshToken == "" {
		t.Fatalf("refresh response missing rotated refresh_token: %s", rBody)
	}
	if rts.RefreshToken == ts.RefreshToken {
		t.Fatalf("rotation failed: refresh_token unchanged after refresh grant")
	}
	if rts.AccessToken == ts.AccessToken {
		t.Fatalf("refresh did not mint a new access_token")
	}
	verifyAccessTokenLocally(t, env.base, rts.AccessToken, env.base, env.subject)
	assertAccessTokenAudiences(t, rts.AccessToken, []string{clientID, "api://" + clientID})
	t.Logf("refresh OK: new access_token + rotated refresh_token (rt→rt2)")
}

// TestOIDCRefreshRolesUpdated proves the Task-5 fresh-roles-on-refresh seam:
// after the initial token (roles claim == ["user"]), GRANTING "admin" to the
// identity and then exercising the refresh_token grant yields a NEW access-token
// JWT whose "roles" claim now contains "admin". Without the refresh-roles
// injection in the Token handler, fosite would re-sign the frozen authorize-time
// roles (["user"]) and this test would FAIL.
func TestOIDCRefreshRolesUpdated(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Initial authorize → token. The default role is "user", so the access-token
	// roles claim must be exactly ["user"] at this point.
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)
	if ts.RefreshToken == "" {
		t.Fatalf("offline_access requested but no refresh_token: %s", body)
	}
	roles0 := decodeAccessTokenRoles(t, env.base, ts.AccessToken)
	if !rolesContain(roles0, "user") || rolesContain(roles0, "admin") {
		t.Fatalf("initial access token: want roles=[user] (no admin), got %v", roles0)
	}
	t.Logf("initial access token roles = %v", roles0)

	// Grant admin AFTER the authorize-time session was frozen into the refresh
	// token. A pure-delegation Token handler would never see this change.
	if err := env.svc.GrantRole(env.ctx, env.subject, "admin"); err != nil {
		t.Fatalf("GrantRole(admin): %v", err)
	}

	// Refresh grant → the re-minted access token must carry the FRESH roles.
	rStatus, rBody, rts := refreshToken(t, env, clientID, ts.RefreshToken)
	assertValidTokenResponse(t, rStatus, rBody, rts, false)
	roles1 := decodeAccessTokenRoles(t, env.base, rts.AccessToken)
	if !rolesContain(roles1, "admin") {
		t.Fatalf("refresh access token: want roles to include admin after grant, got %v", roles1)
	}
	if !rolesContain(roles1, "user") {
		t.Fatalf("refresh access token: existing role 'user' dropped, got %v", roles1)
	}
	t.Logf("refresh access token roles = %v (admin propagated on refresh)", roles1)
}

// TestOIDCRotationReplayKillsFamily is the rotation/replay defense:
// after rt→rt2 rotation, replaying the OLD rt is rejected AND poisons the whole
// family, so rt2 is then dead too. fosite revokes by request_id, which is reused
// across the refresh chain.
func TestOIDCRotationReplayKillsFamily(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Initial authorize → token to get rt.
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)
	rt := ts.RefreshToken
	if rt == "" {
		t.Fatalf("no refresh_token issued: %s", body)
	}

	// Rotate rt → rt2 (refresh grant does not re-mint id_token).
	rStatus, rBody, rts := refreshToken(t, env, clientID, rt)
	assertValidTokenResponse(t, rStatus, rBody, rts, false)
	rt2 := rts.RefreshToken
	if rt2 == "" || rt2 == rt {
		t.Fatalf("rotation failed: rt2=%q rt=%q", rt2, rt)
	}

	// Replay the OLD rt → must be rejected (invalid_grant). This replay triggers
	// family revocation.
	replayStatus, replayBody, _ := refreshToken(t, env, clientID, rt)
	if replayStatus == http.StatusOK {
		t.Fatalf("replay of old refresh token unexpectedly succeeded: %s", replayBody)
	}
	if !strings.Contains(string(replayBody), "invalid_grant") {
		t.Fatalf("replay of old rt: expected invalid_grant, got %d: %s", replayStatus, replayBody)
	}
	t.Logf("replay of old rt rejected (status=%d invalid_grant) — family poisoned", replayStatus)

	// rt2 must now ALSO be dead (whole family revoked by the replay).
	rt2Status, rt2Body, _ := refreshToken(t, env, clientID, rt2)
	if rt2Status == http.StatusOK {
		t.Fatalf("rt2 still usable after family revocation: %s", rt2Body)
	}
	t.Logf("rt2 also dead after replay (status=%d) — family revocation confirmed", rt2Status)
}

// TestOIDCRevoke is the RFC 7009 path: a freshly minted refresh
// token rt3 is revoked via /oauth2/revoke (HTTP 200), after which it is unusable
// at the token endpoint.
func TestOIDCRevoke(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Fresh authorize → token to get rt3.
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)
	rt3 := ts.RefreshToken
	if rt3 == "" {
		t.Fatalf("no refresh_token issued: %s", body)
	}

	// POST /oauth2/revoke (token=rt3&client_id=demo-web) → 200.
	revForm := url.Values{
		"token":     {rt3},
		"client_id": {clientID},
	}
	revReq, err := http.NewRequestWithContext(env.ctx, http.MethodPost,
		env.base+"/oauth2/revoke", strings.NewReader(revForm.Encode()))
	if err != nil {
		t.Fatalf("build revoke request: %v", err)
	}
	revReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revResp, err := http.DefaultClient.Do(revReq)
	if err != nil {
		t.Fatalf("revoke request: %v", err)
	}
	defer revResp.Body.Close()
	revBody, _ := io.ReadAll(revResp.Body)
	if revResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", revResp.StatusCode, revBody)
	}
	t.Logf("revoke OK: HTTP 200 for rt3")

	// rt3 must now be unusable.
	rStatus, rBody, _ := refreshToken(t, env, clientID, rt3)
	if rStatus == http.StatusOK {
		t.Fatalf("refresh with revoked rt3 unexpectedly succeeded: %s", rBody)
	}
	t.Logf("revoked rt3 rejected at token endpoint (status=%d)", rStatus)
}

// TestOIDCEndSessionPassiveLogout is the RP-initiated logout +
// passive-logout path: a refresh token rt4 bound to the login session is revoked
// when /oauth2/end_session clears that session, AND the IdP session itself is
// gone afterwards. Requires svc.SetRefreshRevoker(store) in setup.
func TestOIDCEndSessionPassiveLogout(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	const scope = "openid profile email roles offline_access"

	// Authorize → token to get rt4 (bound to env.sid via Session.IdPSessionID).
	p := newPKCE(t)
	code := authorizeForCode(t, env, clientID, scope, p)
	status, body, ts := exchangeCode(t, env, clientID, code, p.verifier)
	assertValidTokenResponse(t, status, body, ts, true)
	rt4 := ts.RefreshToken
	if rt4 == "" {
		t.Fatalf("no refresh_token issued: %s", body)
	}

	// Sanity: the session is currently valid (Me resolves).
	if _, err := env.svc.Me(env.ctx, env.sid); err != nil {
		t.Fatalf("precondition: session should be valid before logout: %v", err)
	}

	// GET /oauth2/end_session with the id_session cookie → 200 {"logged_out":true}.
	esReq, err := http.NewRequestWithContext(env.ctx, http.MethodGet,
		env.base+"/oauth2/end_session", nil)
	if err != nil {
		t.Fatalf("build end_session request: %v", err)
	}
	esReq.Header.Set("Cookie", "id_session="+env.sid)
	esResp, err := http.DefaultClient.Do(esReq)
	if err != nil {
		t.Fatalf("end_session request: %v", err)
	}
	defer esResp.Body.Close()
	esBody, _ := io.ReadAll(esResp.Body)
	if esResp.StatusCode != http.StatusOK {
		t.Fatalf("end_session: expected 200, got %d: %s", esResp.StatusCode, esBody)
	}
	var es struct {
		LoggedOut bool `json:"logged_out"`
	}
	if err := json.Unmarshal(esBody, &es); err != nil {
		t.Fatalf("decode end_session response: %v (%s)", err, esBody)
	}
	if !es.LoggedOut {
		t.Fatalf("end_session: logged_out != true: %s", esBody)
	}
	t.Logf("end_session OK: 200 logged_out=true")

	// Passive logout: rt4 (session-bound) must now be revoked → unusable.
	rStatus, rBody, _ := refreshToken(t, env, clientID, rt4)
	if rStatus == http.StatusOK {
		t.Fatalf("refresh with rt4 after end_session unexpectedly succeeded (passive logout failed): %s", rBody)
	}
	t.Logf("passive logout OK: rt4 rejected at token endpoint (status=%d)", rStatus)

	// The IdP session itself must be gone: Me returns an error, and a fresh
	// authorize with that cookie now redirects to /login.
	if _, err := env.svc.Me(env.ctx, env.sid); err == nil {
		t.Fatalf("end_session: IdP session still resolves via Me after logout")
	}
	p2 := newPKCE(t)
	authURL := env.base + "/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {callbackURI},
		"scope":                 {scope},
		"state":                 {"xyz-test-state-123"},
		"code_challenge":        {p2.challenge},
		"code_challenge_method": {"S256"},
	}.Encode()
	authReq, err := http.NewRequestWithContext(env.ctx, http.MethodGet, authURL, nil)
	if err != nil {
		t.Fatalf("build post-logout authorize request: %v", err)
	}
	authReq.Header.Set("Cookie", "id_session="+env.sid)
	authResp, err := noRedirectClient().Do(authReq)
	if err != nil {
		t.Fatalf("post-logout authorize request: %v", err)
	}
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusFound && authResp.StatusCode != http.StatusSeeOther {
		ab, _ := io.ReadAll(authResp.Body)
		t.Fatalf("post-logout authorize: expected redirect to login, got %d: %s", authResp.StatusCode, ab)
	}
	if loc := authResp.Header.Get("Location"); !strings.HasPrefix(loc, env.base+"/login") {
		t.Fatalf("post-logout authorize: expected redirect to %s/login..., got %q", env.base, loc)
	}
	t.Logf("post-logout authorize correctly redirects to /login — IdP session cleared")
}

func TestOIDCEndSessionRedirectsToAllowedPostLogoutURI(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)

	postLogoutURI := "http://127.0.0.1/after-logout"
	esReq, err := http.NewRequestWithContext(env.ctx, http.MethodGet,
		env.base+"/oauth2/end_session?"+url.Values{
			"client_id":                {clientID},
			"post_logout_redirect_uri": {postLogoutURI},
		}.Encode(), nil)
	if err != nil {
		t.Fatalf("build end_session redirect request: %v", err)
	}
	esReq.Header.Set("Cookie", "id_session="+env.sid)

	esResp, err := noRedirectClient().Do(esReq)
	if err != nil {
		t.Fatalf("end_session redirect request: %v", err)
	}
	defer esResp.Body.Close()

	if esResp.StatusCode != http.StatusFound && esResp.StatusCode != http.StatusSeeOther {
		esBody, _ := io.ReadAll(esResp.Body)
		t.Fatalf("end_session redirect: expected 302/303, got %d: %s", esResp.StatusCode, esBody)
	}
	if loc := esResp.Header.Get("Location"); loc != postLogoutURI {
		t.Fatalf("end_session redirect: Location = %q, want %q", loc, postLogoutURI)
	}
	if _, err := env.svc.Me(env.ctx, env.sid); err == nil {
		t.Fatalf("end_session redirect: IdP session still resolves via Me after logout")
	}
}

func TestOIDCEndSessionRejectsUnregisteredPostLogoutURI(t *testing.T) {
	const clientID = "demo-web"
	env := setupE2E(t, clientID)
	req, err := http.NewRequestWithContext(env.ctx, http.MethodGet,
		env.base+"/oauth2/end_session?"+url.Values{
			"client_id":                {clientID},
			"post_logout_redirect_uri": {"http://127.0.0.1/not-registered"},
		}.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "id_session="+env.sid)

	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Location") != "" {
		t.Fatalf("unregistered logout URI accepted: status=%d location=%q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func assertAccessTokenAudiences(t *testing.T, token string, want []string) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("access token is not a compact JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode access-token claims: %v", err)
	}
	var claims struct {
		Audience []string `json:"aud"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("unmarshal access-token claims: %v", err)
	}
	if !slices.Equal(claims.Audience, want) {
		t.Fatalf("access-token aud = %v, want %v", claims.Audience, want)
	}
}

// verifyAccessTokenLocally simulates a resource server: fetches JWKS, verifies
// the RS256 signature offline, and asserts core claims. Mirrors the PoC's
// verifyAccessTokenLocally but adapted for this service's JWKS endpoint path.
func verifyAccessTokenLocally(t *testing.T, idpURL, accessToken, expectedIss, expectedSub string) {
	t.Helper()

	// Fetch JWKS (a resource server would cache this).
	jwksResp, err := http.Get(idpURL + "/oauth2/jwks.json") //nolint:noctx
	if err != nil {
		t.Fatalf("fetch jwks: %v", err)
	}
	defer jwksResp.Body.Close()
	var jwks jose.JSONWebKeySet
	if err := json.NewDecoder(jwksResp.Body).Decode(&jwks); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}
	if len(jwks.Keys) == 0 {
		t.Fatalf("jwks: no keys in response")
	}

	// Parse the JWT header: confirm RS256, select key by kid.
	parsed, err := josejwt.ParseSigned(accessToken)
	if err != nil {
		t.Fatalf("parse signed access token: %v", err)
	}
	if len(parsed.Headers) == 0 {
		t.Fatalf("access token: no JOSE headers")
	}
	alg := parsed.Headers[0].Algorithm
	if alg != string(jose.RS256) {
		t.Fatalf("access token: alg = %q, want RS256", alg)
	}
	kid := parsed.Headers[0].KeyID
	matched := jwks.Key(kid)
	if len(matched) == 0 {
		t.Fatalf("no JWKS key matches token kid %q", kid)
	}

	// Verify signature using ONLY the public key from JWKS (offline, no IdP call).
	var claims map[string]interface{}
	if err := parsed.Claims(matched[0].Key, &claims); err != nil {
		t.Fatalf("LOCAL RS256 signature verification FAILED: %v", err)
	}

	// Validate core claims locally.
	if claims["iss"] != expectedIss {
		t.Fatalf("iss = %v, want %s", claims["iss"], expectedIss)
	}
	if claims["sub"] != expectedSub {
		t.Fatalf("sub = %v, want %s", claims["sub"], expectedSub)
	}
	expF, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp claim missing or not a number: %v", claims["exp"])
	}
	if time.Unix(int64(expF), 0).Before(time.Now()) {
		t.Fatalf("access token already expired (exp=%v)", expF)
	}
	scope, _ := claims["scope"].(string)
	if !strings.Contains(scope, "openid") {
		t.Fatalf("access token scope claim missing 'openid': %q", scope)
	}

	t.Logf("LOCAL JWKS verify OK: alg=%s kid=%s iss=%v sub=%v scope=%q",
		alg, kid, claims["iss"], claims["sub"], scope)
}

// decodeAccessTokenRoles fetches the IdP JWKS, RS256-verifies the access token
// offline (resource-server style), and returns its "roles" claim as a []string.
// It mirrors verifyAccessTokenLocally's verify path but extracts the roles claim.
func decodeAccessTokenRoles(t *testing.T, idpURL, accessToken string) []string {
	t.Helper()

	jwksResp, err := http.Get(idpURL + "/oauth2/jwks.json") //nolint:noctx
	if err != nil {
		t.Fatalf("fetch jwks: %v", err)
	}
	defer jwksResp.Body.Close()
	var jwks jose.JSONWebKeySet
	if err := json.NewDecoder(jwksResp.Body).Decode(&jwks); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}

	parsed, err := josejwt.ParseSigned(accessToken)
	if err != nil {
		t.Fatalf("parse signed access token: %v", err)
	}
	if len(parsed.Headers) == 0 {
		t.Fatalf("access token: no JOSE headers")
	}
	matched := jwks.Key(parsed.Headers[0].KeyID)
	if len(matched) == 0 {
		t.Fatalf("no JWKS key matches token kid %q", parsed.Headers[0].KeyID)
	}
	var claims map[string]interface{}
	if err := parsed.Claims(matched[0].Key, &claims); err != nil {
		t.Fatalf("LOCAL RS256 signature verification FAILED: %v", err)
	}

	raw, ok := claims["roles"]
	if !ok {
		t.Fatalf("access token has no 'roles' claim: %v", claims)
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("access token 'roles' claim is not an array: %T (%v)", raw, raw)
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("access token 'roles' element not a string: %T (%v)", v, v)
		}
		out = append(out, s)
	}
	return out
}

// rolesContain reports whether roles includes target.
func rolesContain(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// randPKCEVerifier generates a random PKCE code verifier (48 bytes → 64 base64url chars).
func randPKCEVerifier(t *testing.T) string {
	t.Helper()
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
