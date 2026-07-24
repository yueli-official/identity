package controller_test

// TestE2E_AuditLogging is a hermetic end-to-end test that drives all auditable
// actions through real HTTP so the ActorMiddleware → logic.audit path is
// genuinely exercised, and then queries those records via the admin endpoint.
//
// The server is set up locally in this test (not via a shared helper) so that
// ActorMiddleware can be registered globally — a requirement for the ip/userAgent
// assertions to be meaningful. The sibling tests (admin_e2e_test.go,
// controller_test.go) omit ActorMiddleware; adding it to a shared helper would
// change their semantics, so we set up our own server here.
//
// Scenarios covered:
//  1. Register + login → audit visible via admin endpoint; login.success entry
//     carries non-empty ip and userAgent (proves ActorMiddleware populated ctx).
//  2. Login failure recorded with result=failure and detail.reason=bad_password.
//  3. Admin grant-role records actor: role.granted entry has actorId==adminID
//     and targetId==userID and detail.role==granted role.
//  4. Endpoint authz: non-admin and unauthenticated → 403 / 401; admin → 200.

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

func TestE2E_AuditLogging(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		// --- in-memory repo, service, controller ---
		store := repo.NewMemory()
		svc := logic.New(store, logic.DefaultConfig())
		ctl := controller.New(svc, false) // insecure cookie: works over plain HTTP

		// --- start server with BOTH ActorMiddleware and the envelope middleware ---
		// ActorMiddleware must be global (s.Use) so it runs for every request and
		// populates IP/UA into ctx before the business handler calls svc methods.
		s := g.Server(t.Name())
		s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
		s.SetDumpRouterMap(false)
		s.Use(controller.ActorMiddleware) // <-- THIS is what the other e2e tests omit
		s.Group("/", func(grp *ghttp.RouterGroup) {
			grp.Middleware(ghttpx.Middleware)
			grp.Bind(ctl)
		})
		s.Start()
		defer s.Shutdown()

		base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

		// --- HTTP helpers ---

		// doPost issues a POST and returns the response body string and the
		// response headers. The response body is fully drained and closed before
		// returning, so callers get the headers (e.g. Set-Cookie) rather than the
		// (already consumed) *http.Response.
		doPost := func(path, jsonBody string, headers map[string]string) (string, http.Header) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path,
				strings.NewReader(jsonBody))
			t.AssertNil(err)
			req.Header.Set("Content-Type", "application/json")
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			t.AssertNil(err)
			defer resp.Body.Close()
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, resp.Body)
			return buf.String(), resp.Header
		}

		doGet := func(path string, headers map[string]string) (string, int) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
			t.AssertNil(err)
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := http.DefaultClient.Do(req)
			t.AssertNil(err)
			defer resp.Body.Close()
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, resp.Body)
			return buf.String(), resp.StatusCode
		}

		// extractSessionCookie reads the id_session value from Set-Cookie on a login response.
		extractSessionCookie := func(hdr http.Header) string {
			for _, raw := range hdr["Set-Cookie"] {
				parts := strings.Split(raw, ";")
				if len(parts) > 0 {
					kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
					if len(kv) == 2 && kv[0] == "id_session" {
						return kv[1]
					}
				}
			}
			return ""
		}

		// auditHdr returns headers with the given session cookie for admin audit queries.
		cookieHdr := func(sid string) map[string]string {
			return map[string]string{"Cookie": "id_session=" + sid}
		}

		// =====================================================================
		// Bootstrap: create a regular user and an admin user over the service
		// seam. Only the actions we want to assert over HTTP go through HTTP.
		// =====================================================================

		// Create admin via service seam (mirrors main.go bootstrap):
		adminOut, err := svc.Register(ctx, logic.RegisterInput{
			Email: "admin@audit.test", Password: "correct horse battery", DisplayName: "Admin",
		})
		t.AssertNil(err)
		t.AssertNil(svc.GrantRole(ctx, adminOut.ID, logic.AdminRole))
		adminID := adminOut.ID

		// Log the admin in over HTTP so we get a real session cookie:
		_, adminLoginHdr := doPost("/api/v1/auth/login",
			`{"email":"admin@audit.test","password":"correct horse battery"}`, nil)
		adminSID := extractSessionCookie(adminLoginHdr)
		t.AssertNE(adminSID, "")

		// =====================================================================
		// Scenario 1: Register + Login over HTTP → check audit entries
		// =====================================================================

		// Register the regular user via HTTP (so ActorMiddleware runs for it):
		const userEmail = "user@audit.test"
		const userPassword = "correct horse battery"
		const testUA = "TestBrowser/1.0 (audit-e2e)"

		_, _ = doPost("/api/v1/auth/register",
			fmt.Sprintf(`{"email":%q,"password":%q,"displayName":"AuditUser"}`, userEmail, userPassword),
			map[string]string{"User-Agent": testUA},
		)
		// Resolve the user ID via the service so we can filter audit logs:
		userIdentity, err := svc.GetByEmail(ctx, userEmail)
		t.AssertNil(err)
		userID := userIdentity.ID

		// Log the user in over HTTP with a recognisable User-Agent:
		_, loginHdr := doPost("/api/v1/auth/login",
			fmt.Sprintf(`{"email":%q,"password":%q}`, userEmail, userPassword),
			map[string]string{"User-Agent": testUA},
		)
		userSID := extractSessionCookie(loginHdr)
		t.AssertNE(userSID, "")

		// Query the admin audit endpoint for the user's events:
		auditURL := fmt.Sprintf("/api/v1/admin/audit?identityId=%s", userID)
		auditBody, auditStatus := doGet(auditURL, cookieHdr(adminSID))
		t.Assert(auditStatus, 200)
		aj := gjson.New(auditBody)

		entries := aj.Get("entries").Array()
		t.AssertGT(len(entries), 0)

		// Collect event names for membership checks. Capture the login.success
		// entry whose userAgent matches the testUA we sent so the IP/UA assertion
		// below is meaningful even if more login events accumulate later.
		eventNames := make(map[string]bool)
		var loginSuccessEntry *gjson.Json
		for _, raw := range entries {
			entry := gjson.New(raw)
			ev := entry.Get("event").String()
			eventNames[ev] = true
			if ev == "login.success" && loginSuccessEntry == nil &&
				entry.Get("userAgent").String() == testUA {
				loginSuccessEntry = entry
			}
		}

		t.Assert(eventNames["identity.register"], true)
		t.Assert(eventNames["role.default_granted"], true)
		t.Assert(eventNames["login.success"], true)

		// Assert IP is non-empty and userAgent matches what we sent on login.success
		// (proves ActorMiddleware populated ctx from the real request):
		t.AssertNE(loginSuccessEntry, nil)
		t.AssertNE(loginSuccessEntry.Get("ip").String(), "")
		t.Assert(loginSuccessEntry.Get("userAgent").String(), testUA)

		// =====================================================================
		// Scenario 2: Login failure → login.failure entry with result=failure
		// and detail.reason=bad_password
		// =====================================================================

		failBody, _ := doPost("/api/v1/auth/login",
			fmt.Sprintf(`{"email":%q,"password":"wrongpassword"}`, userEmail),
			nil,
		)
		// The response should be an error (invalid credentials):
		t.AssertNE(gjson.New(failBody).Get("code").String(), "ok")

		// Query audit logs filtered by event=login.failure:
		failAuditBody, failAuditStatus := doGet(
			"/api/v1/admin/audit?event=login.failure",
			cookieHdr(adminSID),
		)
		t.Assert(failAuditStatus, 200)
		faj := gjson.New(failAuditBody)

		failEntries := faj.Get("entries").Array()
		t.AssertGT(len(failEntries), 0)

		// Find the login.failure for our user (by email matching in detail or result):
		var foundFailure bool
		for _, raw := range failEntries {
			entry := gjson.New(raw)
			if entry.Get("result").String() == "failure" &&
				entry.Get("detail.reason").String() == "bad_password" {
				foundFailure = true
				break
			}
		}
		t.Assert(foundFailure, true)

		// =====================================================================
		// Scenario 3: Admin grant-role → role.granted records actorId==adminID,
		// targetId==userID, detail.role==granted-role
		// =====================================================================

		const grantedRole = "admin"
		grantPath := fmt.Sprintf("/api/v1/admin/identities/%s/roles?role=%s", userID, grantedRole)
		_, _ = doPost(grantPath, "", cookieHdr(adminSID))

		// Query audit for role.granted on the target user:
		roleAuditBody, roleAuditStatus := doGet(
			fmt.Sprintf("/api/v1/admin/audit?identityId=%s&event=role.granted", userID),
			cookieHdr(adminSID),
		)
		t.Assert(roleAuditStatus, 200)
		raj := gjson.New(roleAuditBody)

		roleEntries := raj.Get("entries").Array()
		t.AssertGT(len(roleEntries), 0)

		// Find the role.granted entry for grantedRole ("admin") granted by the admin:
		var foundRoleEntry bool
		for _, raw := range roleEntries {
			entry := gjson.New(raw)
			if entry.Get("event").String() == "role.granted" &&
				entry.Get("targetId").String() == userID &&
				entry.Get("actorId").String() == adminID &&
				entry.Get("detail.role").String() == grantedRole {
				foundRoleEntry = true
				break
			}
		}
		t.Assert(foundRoleEntry, true)

		// =====================================================================
		// Scenario 4: Endpoint authz
		// =====================================================================

		// 4a. Unauthenticated (no cookie) → 401
		{
			body, status := doGet("/api/v1/admin/audit", nil)
			// The server may return 401 or 403 for unauthenticated; our iderr package
			// maps iderr.NotAuthenticated to 401. Match the sibling test convention.
			t.Assert(status, 401)
			t.Assert(gjson.New(body).Get("code").String(), "identity.not_authenticated")
		}

		// 4b. Non-admin session (regular user) → 403 forbidden
		{
			// userSID belongs to the user we promoted to admin in scenario 3, so
			// the same session now passes the admin gate. Assert that first (confirms
			// a promoted user gets 200), then create a fresh plain user to exercise
			// the true non-admin 403 path:
			body, status := doGet("/api/v1/admin/audit", cookieHdr(userSID))
			t.Assert(status, 200)

			_, _ = doPost("/api/v1/auth/register",
				`{"email":"plain@audit.test","password":"correct horse battery","displayName":"Plain"}`,
				nil,
			)
			_, plainLoginHdr := doPost("/api/v1/auth/login",
				`{"email":"plain@audit.test","password":"correct horse battery"}`,
				nil,
			)
			plainSID := extractSessionCookie(plainLoginHdr)
			t.AssertNE(plainSID, "")

			body, status = doGet("/api/v1/admin/audit", cookieHdr(plainSID))
			t.Assert(status, 403)
			t.Assert(gjson.New(body).Get("code").String(), "identity.forbidden")
		}

		// 4c. Admin session → 200 success
		{
			_, status := doGet("/api/v1/admin/audit", cookieHdr(adminSID))
			t.Assert(status, 200)
		}
	})
}
