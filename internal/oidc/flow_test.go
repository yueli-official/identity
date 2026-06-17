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
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

// TestOIDCFlow is the milestone-③ acceptance test: hermetic end-to-end
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
		Email: "u@e.com", Password: "longenough123", DisplayName: "U",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	loginOut, err := svc.Login(ctx, logic.LoginInput{
		Email: "u@e.com", Password: "longenough123", IP: "127.0.0.1",
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
	store := oidc.NewStore(r)
	provider := oidc.NewProvider(store, oidc.Config{
		Issuer:       base,
		GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
	}, mgr.KeyGetter)
	ctl := controller.NewOIDC(provider, mgr, svc, base, base+"/login")

	// -----------------------------------------------------------------------
	// 5. Start GoFrame server on the pre-chosen port (NO ghttpx.Middleware)
	// -----------------------------------------------------------------------
	s := g.Server(t.Name())
	s.SetPort(port)
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
	// 9. Local JWKS verification of the access token (THE milestone-③ proof)
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

// randPKCEVerifier generates a random PKCE code verifier (48 bytes → 64 base64url chars).
func randPKCEVerifier(t *testing.T) string {
	t.Helper()
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
