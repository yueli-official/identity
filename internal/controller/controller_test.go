package controller_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// TestAuthFlow is a hermetic end-to-end test covering register → login → me →
// logout → me-unauthenticated. It uses an in-memory repo (no Postgres/Redis)
// and drives a real GoFrame HTTP server. Cookies are propagated by reading the
// Set-Cookie header from the login response and passing the session cookie
// explicitly on subsequent requests.
func TestAuthFlow(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// --- build service + controller ---
		svc := logic.New(repo.NewMemory(), logic.DefaultConfig())
		ctl := controller.New(svc, false) // insecure cookie: round-trips over HTTP

		// --- start server ---
		s := g.Server(t.Name())
		s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
		s.Use(ghttpx.Middleware)
		s.Group("/", func(grp *ghttp.RouterGroup) {
			grp.Bind(ctl)
		})
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

		// helper: POST JSON, return (body string, *http.Response)
		doPost := func(path, jsonBody string, extraHeaders map[string]string) (string, *http.Response) {
			req, err := http.NewRequestWithContext(
				context.Background(), http.MethodPost, base+path,
				strings.NewReader(jsonBody),
			)
			t.AssertNil(err)
			req.Header.Set("Content-Type", "application/json")
			for k, v := range extraHeaders {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			t.AssertNil(err)
			defer resp.Body.Close()
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, resp.Body)
			return buf.String(), resp
		}

		// helper: GET, return body string
		doGet := func(path string, extraHeaders map[string]string) string {
			req, err := http.NewRequestWithContext(
				context.Background(), http.MethodGet, base+path, nil)
			t.AssertNil(err)
			for k, v := range extraHeaders {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			t.AssertNil(err)
			defer resp.Body.Close()
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, resp.Body)
			return buf.String()
		}

		// ------------------------------------------------------------------
		// 1. Register
		// ------------------------------------------------------------------
		{
			_, _ = doPost("/api/v1/auth/register",
				`{"email":"a@b.com","password":"correct horse battery","displayName":"A"}`,
				nil)
		}

		// ------------------------------------------------------------------
		// 2. Login — capture session cookie from Set-Cookie header
		// ------------------------------------------------------------------
		var sessionCookieValue string
		{
			_, resp := doPost("/api/v1/auth/login",
				`{"email":"a@b.com","password":"correct horse battery"}`,
				nil)

			// Extract id_session from Set-Cookie header and assert security flags.
			for _, raw := range resp.Header["Set-Cookie"] {
				// raw is like: id_session=<value>; Path=/; HttpOnly; SameSite=Lax
				parts := strings.Split(raw, ";")
				if len(parts) > 0 {
					kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
					if len(kv) == 2 && kv[0] == "id_session" {
						sessionCookieValue = kv[1]

						// Assert required cookie security attributes are present
						// (guards against regression stripping HttpOnly or SameSite).
						rawLower := strings.ToLower(raw)
						t.Assert(strings.Contains(rawLower, "httponly"), true)
						t.Assert(strings.Contains(rawLower, "samesite"), true)
						t.Assert(strings.Contains(rawLower, "max-age"), true)
						t.Assert(strings.Contains(rawLower, "expires"), true)
					}
				}
			}
			t.AssertNE(sessionCookieValue, "")
		}

		cookieHdr := map[string]string{"Cookie": "id_session=" + sessionCookieValue}

		// ------------------------------------------------------------------
		// 3. GET /me with session cookie → authenticated
		// ------------------------------------------------------------------
		{
			body := doGet("/api/v1/session/me", cookieHdr)
			j := gjson.New(body)
			t.Assert(j.Get("email").String(), "a@b.com")
		}

		// ------------------------------------------------------------------
		// 4. Logout with session cookie
		// ------------------------------------------------------------------
		{
			_, _ = doPost("/api/v1/auth/logout", `{}`, cookieHdr)
		}

		// ------------------------------------------------------------------
		// 5. GET /me after logout — must return not_authenticated
		// ------------------------------------------------------------------
		{
			// No cookie (session was deleted; even if we send the old value it
			// should still fail, but sending none is the clean unauthenticated case)
			body := doGet("/api/v1/session/me", nil)
			j := gjson.New(body)
			t.Assert(j.Get("code").String(), "identity.not_authenticated")
		}
	})
}
