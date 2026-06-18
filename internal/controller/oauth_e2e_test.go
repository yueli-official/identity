package controller_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oauthlogin"
	"platform/services/identity/internal/repo"
)

// fakeProvider is a test-local oauthlogin.Provider that returns canned profile
// data, standing in for the real Google client (whose HTTP layer is unit-tested
// against httptest in internal/oauthlogin/google_test.go). It lets the controller
// e2e exercise the full redirect dance hermetically, without any network.
type fakeProvider struct {
	authBase string          // base URL the AuthorizeURL points at (a dummy)
	user     oauthlogin.UserInfo
}

func (f *fakeProvider) Name() string { return "google" }

func (f *fakeProvider) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", "fake-cid")
	q.Set("response_type", "code")
	q.Set("state", state)
	return f.authBase + "/o/oauth2/v2/auth?" + q.Encode()
}

func (f *fakeProvider) ExchangeCode(_ context.Context, code string) (string, error) {
	if code == "" {
		return "", fmt.Errorf("empty code")
	}
	return "fake-access-token", nil
}

func (f *fakeProvider) FetchUserInfo(_ context.Context, accessToken string) (oauthlogin.UserInfo, error) {
	if accessToken != "fake-access-token" {
		return oauthlogin.UserInfo{}, fmt.Errorf("bad token")
	}
	return f.user, nil
}

var _ oauthlogin.Provider = (*fakeProvider)(nil)

// TestE2E_GoogleLogin is the hermetic end-to-end Google login flow at the
// controller level with a FAKE provider (no network). It asserts:
//   - GET /start → 302 to AuthorizeURL + g_oauth_state Set-Cookie
//   - GET /callback?code&state (with state cookie) → 302 Location == return_to +
//     id_session Set-Cookie
//   - GET /api/v1/session/me with id_session → 200 + correct email
//   - returning user (same ProviderUID) → same identity id, fresh session
func TestE2E_GoogleLogin(t *testing.T) {
	const stateSecret = "0123456789abcdef0123456789abcdef"
	const returnTo = "/profile"

	r := repo.NewMemory()
	svc := logic.New(r, logic.DefaultConfig())

	fp := &fakeProvider{
		authBase: "https://accounts.example.test",
		user: oauthlogin.UserInfo{
			ProviderUID: "sub-1", Email: "u@example.com",
			EmailVerified: true, DisplayName: "U",
		},
	}

	// Build controllers directly: the OAuth raw controller + the business
	// controller (for /session/me). secureCookie=false so cookies round-trip
	// over plain HTTP.
	oauthCtl := controller.NewOAuth(svc, fp, false, []byte(stateSecret), "http://127.0.0.1/login")
	authCtl := controller.New(svc, false)

	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
	s.SetDumpRouterMap(false)
	// Business API: gokit envelope middleware for /session/me.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(ghttpx.Middleware)
		grp.Bind(authCtl)
	})
	// OAuth raw redirect handlers: NO envelope middleware (mirrors main.go).
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/api/v1/auth/oauth/google/start", oauthCtl.GoogleStart)
		grp.GET("/api/v1/auth/oauth/google/callback", oauthCtl.GoogleCallback)
	})
	s.Start()
	defer s.Shutdown()

	base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

	// Cookie-jar client that does NOT auto-follow redirects, so every hop is
	// asserted explicitly (jar still records Set-Cookie for later requests).
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// --- Step 1: GET /start → 302 to AuthorizeURL + g_oauth_state cookie -----
	startResp, err := client.Get(base + "/api/v1/auth/oauth/google/start?return_to=" + url.QueryEscape(returnTo))
	if err != nil {
		t.Fatalf("start request: %v", err)
	}
	startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound && startResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("start: expected 302/303, got %d", startResp.StatusCode)
	}
	authLoc := startResp.Header.Get("Location")
	if !strings.HasPrefix(authLoc, fp.authBase) {
		t.Fatalf("start: Location should be the AuthorizeURL, got %q", authLoc)
	}
	if !hasSetCookie(startResp, oauthStateCookieName) {
		t.Fatalf("start: missing %s Set-Cookie; got %v", oauthStateCookieName, startResp.Header["Set-Cookie"])
	}
	// Extract the signed state the provider was handed.
	authURL, err := url.Parse(authLoc)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatalf("start: AuthorizeURL has no state param: %s", authLoc)
	}

	// --- Step 2: GET /callback?code&state (state cookie attached via jar) ----
	cbURL := base + "/api/v1/auth/oauth/google/callback?" + url.Values{
		"code":  {"good-code"},
		"state": {state},
	}.Encode()
	cbResp, err := client.Get(cbURL)
	if err != nil {
		t.Fatalf("callback request: %v", err)
	}
	cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusFound && cbResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("callback: expected 302/303, got %d (Location=%q)", cbResp.StatusCode, cbResp.Header.Get("Location"))
	}
	if loc := cbResp.Header.Get("Location"); loc != returnTo {
		t.Fatalf("callback: Location = %q, want return_to %q", loc, returnTo)
	}
	if !hasSetCookie(cbResp, "id_session") {
		t.Fatalf("callback: missing id_session Set-Cookie; got %v", cbResp.Header["Set-Cookie"])
	}

	// --- Step 3: GET /session/me with id_session (jar) → 200 + email --------
	firstID := getMeIdentity(t, client, base, "u@example.com")

	// --- Step 4: returning user — repeat start+callback with same sub --------
	// Fresh jar so the previous id_session does not leak; same fake provider
	// (same ProviderUID "sub-1").
	jar2, _ := cookiejar.New(nil)
	client2 := &http.Client{
		Jar: jar2,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start2, err := client2.Get(base + "/api/v1/auth/oauth/google/start?return_to=" + url.QueryEscape(returnTo))
	if err != nil {
		t.Fatalf("returning start request: %v", err)
	}
	start2.Body.Close()
	state2URL, _ := url.Parse(start2.Header.Get("Location"))
	state2 := state2URL.Query().Get("state")
	if state2 == "" {
		t.Fatalf("returning start: no state in %q", start2.Header.Get("Location"))
	}

	cb2, err := client2.Get(base + "/api/v1/auth/oauth/google/callback?" + url.Values{
		"code":  {"good-code"},
		"state": {state2},
	}.Encode())
	if err != nil {
		t.Fatalf("returning callback request: %v", err)
	}
	cb2.Body.Close()
	if cb2.StatusCode != http.StatusFound && cb2.StatusCode != http.StatusSeeOther {
		t.Fatalf("returning callback: expected redirect, got %d", cb2.StatusCode)
	}
	firstSession := sessionCookieValue(jar, base)
	secondSession := sessionCookieValue(jar2, base)
	if firstSession == "" || secondSession == "" {
		t.Fatalf("expected id_session in both jars, got %q / %q", firstSession, secondSession)
	}
	if firstSession == secondSession {
		t.Fatalf("returning user should mint a FRESH session, got identical %q", firstSession)
	}

	secondID := getMeIdentity(t, client2, base, "u@example.com")
	if firstID != secondID {
		t.Fatalf("returning user (same ProviderUID) should resolve same identity id, got %q vs %q", firstID, secondID)
	}
}

// oauthStateCookieName mirrors the controller's oauthStateCookie const (which is
// unexported and lives in the controller package, not this _test package).
const oauthStateCookieName = "g_oauth_state"

// hasSetCookie reports whether the response sets a cookie with the given name.
func hasSetCookie(resp *http.Response, name string) bool {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}

// sessionCookieValue extracts the id_session value the jar holds for base.
func sessionCookieValue(jar http.CookieJar, base string) string {
	u, _ := url.Parse(base)
	for _, c := range jar.Cookies(u) {
		if c.Name == "id_session" {
			return c.Value
		}
	}
	return ""
}

// getMeIdentity calls GET /api/v1/session/me with the jar's cookies, asserts a
// successful envelope with the expected email, and returns the identity id.
func getMeIdentity(t *testing.T, client *http.Client, base, wantEmail string) string {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/session/me")
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	j := gjson.New(body)
	if code := j.Get("code").String(); code != "ok" {
		t.Fatalf("me: expected code=ok, got %q (body=%s)", code, body)
	}
	if email := j.Get("data.email").String(); email != wantEmail {
		t.Fatalf("me: email = %q, want %q (body=%s)", email, wantEmail, body)
	}
	id := j.Get("data.id").String()
	if id == "" {
		t.Fatalf("me: empty identity id (body=%s)", body)
	}
	return id
}
