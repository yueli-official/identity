package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/yueli-official/identity/internal/controller"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/mailer"
	"github.com/yueli-official/identity/internal/repo"
	identityruntime "github.com/yueli-official/identity/internal/runtime"
)

// capMailer is a test-local mailer.Mailer that records the last verify / reset
// link instead of sending it, so the e2e can extract the opaque token from the
// emailed link and drive the consume endpoints over HTTP.
type capMailer struct {
	verifyTo, verifyLink string
	resetTo, resetLink   string
}

func (c *capMailer) SendVerifyEmail(_ context.Context, to, link string) error {
	c.verifyTo, c.verifyLink = to, link
	return nil
}

func (c *capMailer) SendPasswordReset(_ context.Context, to, link string) error {
	c.resetTo, c.resetLink = to, link
	return nil
}

var _ mailer.Mailer = (*capMailer)(nil)

// tokenFromLink pulls the ?token= query value from an emailed action link.
func tokenFromLink(link string) string {
	u, _ := url.Parse(link)
	return u.Query().Get("token")
}

// TestE2E_Email is the hermetic end-to-end email-verify + password-reset flow at
// the controller level, over real HTTP with a capturing mailer (no SMTP). It
// asserts, in order:
//   - register + login (id_session captured by the cookie jar)
//   - POST /auth/email/verify-request (authed) → 200; mailer caught a
//     /verify-email?token= link → POST /auth/email/verify {token} → 200 →
//     /session/me reports emailVerified=true; repo agrees
//   - POST /auth/password/forgot {known} → 200 with a /reset?token= link;
//     {unknown} → also 200 and no mail captured (enumeration guard)
//   - POST /auth/password/reset {token,newPassword} → 200; old id_session no
//     longer resolves (/session/me 401); login with new password works, old fails
func TestE2E_Email(t *testing.T) {
	const (
		email   = "u@example.com"
		oldPass = "old password phrase"
		newPass = "new password phrase"
	)

	r := repo.NewMemory()
	svc := logic.New(r, logic.DefaultConfig())
	cm := &capMailer{}
	svc.SetMailer(cm)

	// Business controller only: the four new endpoints + login/register/me all
	// live on *controller.Controller and auto-register via grp.Bind. secureCookie
	// =false so id_session round-trips over plain HTTP (mirrors oauth_e2e_test).
	authCtl := controller.New(svc, false)

	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
	s.SetDumpRouterMap(false)
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(identityruntime.APIMiddleware)
		grp.Bind(authCtl)
	})
	s.Start()
	defer s.Shutdown()

	base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

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

	// --- Step 1: register + login -------------------------------------------
	postJSON(t, client, base+"/api/v1/auth/register", map[string]any{
		"email": email, "password": oldPass, "displayName": "U",
	})
	postJSON(t, client, base+"/api/v1/auth/login", map[string]any{
		"email": email, "password": oldPass,
	})
	userKey := getMeUserKey(t, client, base, email, false)
	oldSession := emailSessionCookie(jar, base)
	if oldSession == "" {
		t.Fatalf("login: no id_session in jar")
	}

	// --- Step 2: email verification -----------------------------------------
	postJSON(t, client, base+"/api/v1/auth/email/verify-request", map[string]any{})
	if cm.verifyTo != email || !strings.Contains(cm.verifyLink, "/verify-email?token=") {
		t.Fatalf("verify-request: bad mail to=%q link=%q", cm.verifyTo, cm.verifyLink)
	}
	postJSON(t, client, base+"/api/v1/auth/email/verify", map[string]any{
		"token": tokenFromLink(cm.verifyLink),
	})
	// /session/me now reports emailVerified=true.
	getMeUserKey(t, client, base, email, true)
	// And the repo agrees.
	got, err := r.GetByUserKey(context.Background(), userKey)
	if err != nil {
		t.Fatalf("GetByUserKey: %v", err)
	}
	if !got.EmailVerified {
		t.Fatal("repo: email_verified should be true after verify")
	}

	// --- Step 3: forgot (known + unknown, no leak) --------------------------
	postJSON(t, client, base+"/api/v1/auth/password/forgot", map[string]any{"email": email})
	if !strings.Contains(cm.resetLink, "/reset?token=") || cm.resetTo != email {
		t.Fatalf("forgot(known): no reset link to=%q link=%q", cm.resetTo, cm.resetLink)
	}
	resetToken := tokenFromLink(cm.resetLink)

	cm.resetTo, cm.resetLink = "", "" // reset the capture before the unknown-email probe
	postJSON(t, client, base+"/api/v1/auth/password/forgot", map[string]any{
		"email": "nobody@example.com",
	})
	if cm.resetLink != "" || cm.resetTo != "" {
		t.Fatalf("forgot(unknown): must NOT send mail, got to=%q link=%q", cm.resetTo, cm.resetLink)
	}

	// --- Step 4: reset → old session dead, new pw works, old fails ----------
	postJSON(t, client, base+"/api/v1/auth/password/reset", map[string]any{
		"token": resetToken, "password": newPass,
	})

	// The old id_session (still in the jar) must no longer resolve.
	meResp, err := client.Get(base + "/api/v1/session/me")
	if err != nil {
		t.Fatalf("me after reset: %v", err)
	}
	meBody, _ := io.ReadAll(meResp.Body)
	meResp.Body.Close()
	if meResp.StatusCode >= 200 && meResp.StatusCode < 300 {
		t.Fatalf("reset: old session should be force-logged-out, /session/me returned %d: %s", meResp.StatusCode, meBody)
	}

	// New password works on a fresh client.
	freshJar, _ := cookiejar.New(nil)
	fresh := &http.Client{Jar: freshJar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	postJSON(t, fresh, base+"/api/v1/auth/login", map[string]any{
		"email": email, "password": newPass,
	})

	// Old password fails.
	failJar, _ := cookiejar.New(nil)
	failClient := &http.Client{Jar: failJar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	failBody := postJSON(t, failClient, base+"/api/v1/auth/login", map[string]any{
		"email": email, "password": oldPass,
	})
	if code := gjson.New(failBody).Get("code").String(); code != "identity.invalid_credentials" {
		t.Fatalf("login(oldPass): code=%q, want identity.invalid_credentials: %s", code, failBody)
	}
}

// postJSON POSTs a JSON body and returns the response body, failing the test on
// transport error. Problem codes are asserted by callers exercising failures.
func postJSON(t *testing.T, client *http.Client, url string, payload map[string]any) []byte {
	t.Helper()
	buf, _ := json.Marshal(payload)
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body
}

// getMeUserKey calls /session/me, asserts email + emailVerified, and returns the
// stable public user key without exposing the internal UUID.
func getMeUserKey(t *testing.T, client *http.Client, base, wantEmail string, wantVerified bool) string {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/session/me")
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	j := gjson.New(body)
	if email := j.Get("email").String(); email != wantEmail {
		t.Fatalf("me: email=%q want %q", email, wantEmail)
	}
	if v := j.Get("emailVerified").Bool(); v != wantVerified {
		t.Fatalf("me: emailVerified=%v want %v (body=%s)", v, wantVerified, body)
	}
	return j.Get("userKey").String()
}

// emailSessionCookie extracts the id_session value the jar holds for base.
func emailSessionCookie(jar http.CookieJar, base string) string {
	u, _ := url.Parse(base)
	for _, c := range jar.Cookies(u) {
		if c.Name == "id_session" {
			return c.Value
		}
	}
	return ""
}
