package controller_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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

// TestE2E_OAuthOnlyUnbindClosedLoop reproduces the reported "Google 登录后点解绑没反应"
// scenario on the real HTTP stack, hermetically (fakeProvider, no network — this
// machine can't reach Google). It proves the full closed loop:
//   - an OAuth-only account (no password) is REFUSED unbinding its sole google
//     credential (identity.last_credential — the correct anti-lockout behavior
//     that surfaced as the second click's error)
//   - after setting an initial password via the new /auth/password/set endpoint,
//     google is no longer the last credential and unbind SUCCEEDS
//   - set-password is initial-only (a second call is refused)
func TestE2E_OAuthOnlyUnbindClosedLoop(t *testing.T) {
	const stateSecret = "0123456789abcdef0123456789abcdef"

	r := repo.NewMemory()
	svc := logic.New(r, logic.DefaultConfig())
	fp := &fakeProvider{
		authBase: "https://accounts.example.test",
		user: oauthlogin.UserInfo{
			ProviderUID: "solo-1", Email: "solo@example.com",
			EmailVerified: true, DisplayName: "Solo",
		},
	}
	oauthCtl := controller.NewOAuth(svc, fp, false, []byte(stateSecret), "http://127.0.0.1/login")
	authCtl := controller.New(svc, false)

	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
	s.SetDumpRouterMap(false)
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(ghttpx.Middleware)
		grp.Bind(authCtl)
	})
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/api/v1/auth/oauth/google/start", oauthCtl.GoogleStart)
		grp.GET("/api/v1/auth/oauth/google/callback", oauthCtl.GoogleCallback)
	})
	s.Start()
	defer s.Shutdown()
	base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// --- Log in via fake Google → an OAuth-only identity (no password) ----------
	startResp, err := client.Get(base + "/api/v1/auth/oauth/google/start?return_to=/")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	startResp.Body.Close()
	stateURL, _ := url.Parse(startResp.Header.Get("Location"))
	state := stateURL.Query().Get("state")
	if state == "" {
		t.Fatalf("start: no state in %q", startResp.Header.Get("Location"))
	}
	cb, err := client.Get(base + "/api/v1/auth/oauth/google/callback?" + url.Values{
		"code": {"good-code"}, "state": {state},
	}.Encode())
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	cb.Body.Close()
	if !hasSetCookie(cb, "id_session") {
		t.Fatalf("callback: no id_session cookie set")
	}

	// --- Precondition: exactly one credential (google), no password -------------
	creds := getEnvelope(t, client, base+"/api/v1/session/credentials")
	if creds.Get("hasPassword").Bool() {
		t.Fatalf("precondition: hasPassword=true, want false (body=%s)", creds)
	}
	if n := len(creds.Get("oauth").Array()); n != 1 {
		t.Fatalf("precondition: oauth count=%d, want 1 (body=%s)", n, creds)
	}

	// --- Unbinding the LAST credential is refused (the bug's correct behavior) ---
	del := deleteEnvelope(t, client, base+"/api/v1/session/credentials/google")
	if code := del.Get("code").String(); code != "identity.last_credential" {
		t.Fatalf("unbind last credential: code=%q, want identity.last_credential (body=%s)", code, del)
	}

	// --- Set an initial password via the new endpoint (no current password) -----
	postJSON(t, client, base+"/api/v1/auth/password/set", map[string]any{"newPassword": "correct horse battery"})

	// --- Now google is no longer the last credential → unbind succeeds ----------
	if creds2 := getEnvelope(t, client, base+"/api/v1/session/credentials"); !creds2.Get("hasPassword").Bool() {
		t.Fatalf("after set: hasPassword=false, want true (body=%s)", creds2)
	}
	deleteEnvelope(t, client, base+"/api/v1/session/credentials/google")

	// --- Final state: google gone, password remains -----------------------------
	creds3 := getEnvelope(t, client, base+"/api/v1/session/credentials")
	if n := len(creds3.Get("oauth").Array()); n != 0 {
		t.Fatalf("final: oauth count=%d, want 0 (body=%s)", n, creds3)
	}
	if !creds3.Get("hasPassword").Bool() {
		t.Fatalf("final: password should remain after unbind (body=%s)", creds3)
	}

	// --- set-password is initial-only: a second call is refused -----------------
	setAgain := postJSON(t, client, base+"/api/v1/auth/password/set", map[string]any{"newPassword": "another password phrase"})
	if code := gjson.New(setAgain).Get("code").String(); code != "identity.password_already_set" {
		t.Fatalf("second set: code=%q, want identity.password_already_set (body=%s)", code, setAgain)
	}
}

// getEnvelope does a GET and returns the parsed response document.
func getEnvelope(t *testing.T, client *http.Client, url string) *gjson.Json {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return gjson.New(body)
}

// deleteEnvelope does a DELETE and returns the parsed response document.
func deleteEnvelope(t *testing.T, client *http.Client, url string) *gjson.Json {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return gjson.New(body)
}
