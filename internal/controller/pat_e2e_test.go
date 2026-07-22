package controller_test

// TestE2E_PAT is a hermetic end-to-end test that drives all PAT lifecycle
// operations through real HTTP so the full stack — middleware, controller,
// logic, in-memory repo — is exercised. No real DB, no network calls.
//
// Scenarios covered:
//  1. Register + login user A; capture session cookie.
//  2. CreatePAT: token starts with "pat_", prefix non-empty, id>0, scopes match.
//     Plaintext is recorded for later verification assertions.
//  3. ListPAT: exactly one entry; raw response body does NOT contain the
//     plaintext token or any "token"/"tokenHash" key (security assertion).
//  4. VerifyPAT (public, no cookie): identityId == user A, scopes match.
//  5. RevokePAT then VerifyPAT → 401 identity.pat_invalid.
//  6. Verify garbage token → 401 identity.pat_invalid;
//     Verify empty Authorization → 401 identity.pat_invalid.
//  7. Cross-user: user B cannot revoke user A's token (404 identity.pat_not_found);
//     the token still appears in A's list afterward (not deleted).
//  8. Audit: store contains pat.created + pat.revoked rows for user A
//     (ActorID == TargetID == A's identity id).

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

func TestE2E_PAT(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()

		// ── hermetic harness: in-memory repo, service, controller ────────────
		store := repo.NewMemory()
		svc := logic.New(store, logic.DefaultConfig())
		ctl := controller.New(svc, false) // insecure cookie: works over plain HTTP

		// ── server with ActorMiddleware + envelope group ──────────────────────
		s := g.Server(t.Name())
		s.SetAddr("127.0.0.1:0") // loopback-only: avoids the Windows Firewall prompt
		s.SetDumpRouterMap(false)
		s.Use(controller.ActorMiddleware)
		s.Group("/", func(grp *ghttp.RouterGroup) {
			grp.Middleware(ghttpx.Middleware)
			grp.Bind(ctl)
		})
		s.Start()
		defer s.Shutdown()

		base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

		// ── HTTP helpers (same pattern as audit_e2e_test.go) ─────────────────

		doPost := func(path, jsonBody string, headers map[string]string) (string, http.Header, int) {
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
			return buf.String(), resp.Header, resp.StatusCode
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

		doDelete := func(path string, headers map[string]string) (string, int) {
			req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base+path, nil)
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

		// extractSessionCookie mirrors audit_e2e_test.go helper.
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

		cookieHdr := func(sid string) map[string]string {
			return map[string]string{"Cookie": "id_session=" + sid}
		}

		// registerAndLogin registers a user over HTTP and logs them in, returning
		// their identity ID (resolved via service seam) and their session cookie.
		registerAndLogin := func(email, password, displayName string) (identityID, sid string) {
			_, _, _ = doPost("/api/v1/auth/register",
				fmt.Sprintf(`{"email":%q,"password":%q,"displayName":%q}`, email, password, displayName),
				nil,
			)

			ident, err := svc.GetByEmail(ctx, email)
			t.AssertNil(err)

			_, loginHdr, _ := doPost("/api/v1/auth/login",
				fmt.Sprintf(`{"email":%q,"password":%q}`, email, password),
				nil,
			)
			sessionID := extractSessionCookie(loginHdr)
			t.AssertNE(sessionID, "")
			return ident.ID, sessionID
		}

		// =====================================================================
		// Step 1: Register + login user A
		// =====================================================================
		userAID, userASID := registerAndLogin("usera@pat.test", "longenough123", "UserA")
		t.AssertNE(userAID, "")
		t.AssertNE(userASID, "")

		// =====================================================================
		// Step 2: CreatePAT for user A
		// =====================================================================
		createBody, _, createStatus := doPost("/api/v1/pat",
			`{"name":"ci","scopes":["read:post","write:post"],"expiresInDays":0}`,
			cookieHdr(userASID),
		)
		t.Assert(createStatus, 200)
		cj := gjson.New(createBody)

		plaintext := cj.Get("token").String()
		tokenPrefix := cj.Get("tokenPrefix").String()
		patID := cj.Get("id").Int64()
		patScopes := cj.Get("scopes").Strings()

		t.Assert(strings.HasPrefix(plaintext, "pat_"), true)
		t.AssertNE(tokenPrefix, "")
		t.AssertGT(patID, int64(0))
		t.Assert(len(patScopes), 2)
		// scopes must match exactly (order may vary; check membership)
		scopeSet := map[string]bool{}
		for _, sc := range patScopes {
			scopeSet[sc] = true
		}
		t.Assert(scopeSet["read:post"], true)
		t.Assert(scopeSet["write:post"], true)

		// =====================================================================
		// Step 3: ListPAT — exactly one entry; raw body does NOT expose plaintext
		// or any token/tokenHash key inside entries.
		// =====================================================================
		listBody, listStatus := doGet("/api/v1/pat", cookieHdr(userASID))
		t.Assert(listStatus, 200)
		lj := gjson.New(listBody)

		entries := lj.Get("entries").Array()
		t.Assert(len(entries), 1)

		entry0 := gjson.New(entries[0])
		t.Assert(entry0.Get("tokenPrefix").String(), tokenPrefix)
		t.Assert(entry0.Get("name").String(), "ci")
		listEntryScopes := entry0.Get("scopes").Strings()
		listScopeSet := map[string]bool{}
		for _, sc := range listEntryScopes {
			listScopeSet[sc] = true
		}
		t.Assert(listScopeSet["read:post"], true)
		t.Assert(listScopeSet["write:post"], true)

		// Security assertion: plaintext must not appear in the raw body,
		// and neither "token" nor "tokenHash" keys should appear in entries.
		t.Assert(strings.Contains(listBody, plaintext), false)
		// Entries should not have a "token" or "tokenHash" field at the JSON level.
		// We check the raw body of the entries portion for these key names.
		// The entries are embedded in the full body; check that the serialized
		// entry objects do not contain "\"token\"" or "\"tokenHash\"" as keys.
		// (We allow "tokenPrefix" since that is the intended public field.)
		rawEntryJSON := gjson.New(entries[0]).MustToJsonString()
		t.Assert(strings.Contains(rawEntryJSON, `"tokenHash"`), false)
		// "token" key check: replace "tokenPrefix" so we don't false-positive on it,
		// then check that the standalone "token" key is absent.
		entryWithoutPrefix := strings.ReplaceAll(rawEntryJSON, `"tokenPrefix"`, `"__prefix__"`)
		t.Assert(strings.Contains(entryWithoutPrefix, `"token"`), false)

		// =====================================================================
		// Step 4: VerifyPAT (public — no cookie needed)
		// =====================================================================
		verifyBody, _, verifyStatus := doPost("/api/v1/pat/verify", "",
			map[string]string{"Authorization": "Bearer " + plaintext},
		)
		t.Assert(verifyStatus, 200)
		vj := gjson.New(verifyBody)
		t.Assert(vj.Get("identityId").String(), userAID)
		verifyScopes := vj.Get("scopes").Strings()
		verifyScopeSet := map[string]bool{}
		for _, sc := range verifyScopes {
			verifyScopeSet[sc] = true
		}
		t.Assert(verifyScopeSet["read:post"], true)
		t.Assert(verifyScopeSet["write:post"], true)

		// =====================================================================
		// Step 5: RevokePAT then VerifyPAT → 401 identity.pat_invalid
		// =====================================================================
		_, revokeStatus := doDelete(
			fmt.Sprintf("/api/v1/pat/%d", patID),
			cookieHdr(userASID),
		)
		t.Assert(revokeStatus, 200)

		// After revocation, the token must not verify.
		verifyAfterRevokeBody, _, verifyAfterRevokeStatus := doPost("/api/v1/pat/verify", "",
			map[string]string{"Authorization": "Bearer " + plaintext},
		)
		t.Assert(verifyAfterRevokeStatus, 401)
		t.Assert(gjson.New(verifyAfterRevokeBody).Get("code").String(), "identity.pat_invalid")

		// =====================================================================
		// Step 6: Garbage token → 401; empty Authorization → 401
		// =====================================================================
		garbageBody, _, garbageStatus := doPost("/api/v1/pat/verify", "",
			map[string]string{"Authorization": "Bearer pat_garbagexxxx"},
		)
		t.Assert(garbageStatus, 401)
		t.Assert(gjson.New(garbageBody).Get("code").String(), "identity.pat_invalid")

		noAuthBody, _, noAuthStatus := doPost("/api/v1/pat/verify", "", nil)
		t.Assert(noAuthStatus, 401)
		t.Assert(gjson.New(noAuthBody).Get("code").String(), "identity.pat_invalid")

		// =====================================================================
		// Step 7: Cross-user isolation — user B cannot revoke user A's token
		// =====================================================================
		// Create user B.
		_, userBSID := registerAndLogin("userb@pat.test", "longenough123", "UserB")

		// Create a fresh PAT for user A so there is a live id for B to attempt.
		freshCreateBody, _, freshCreateStatus := doPost("/api/v1/pat",
			`{"name":"ci-fresh","scopes":["read:post"],"expiresInDays":0}`,
			cookieHdr(userASID),
		)
		t.Assert(freshCreateStatus, 200)
		freshCJ := gjson.New(freshCreateBody)
		freshPATID := freshCJ.Get("id").Int64()
		t.AssertGT(freshPATID, int64(0))

		// User B attempts to delete user A's fresh token.
		crossRevokeBody, crossRevokeStatus := doDelete(
			fmt.Sprintf("/api/v1/pat/%d", freshPATID),
			cookieHdr(userBSID),
		)
		t.Assert(crossRevokeStatus, 404)
		t.Assert(gjson.New(crossRevokeBody).Get("code").String(), "identity.pat_not_found")

		// Confirm the token still exists in user A's list (B did not delete it).
		listAfterBody, listAfterStatus := doGet("/api/v1/pat", cookieHdr(userASID))
		t.Assert(listAfterStatus, 200)
		laj := gjson.New(listAfterBody)
		entriesAfter := laj.Get("entries").Array()
		t.Assert(len(entriesAfter), 1) // exactly the fresh token A created; the revoked one is gone

		// =====================================================================
		// Step 8: Audit — pat.created and pat.revoked rows for user A
		// =====================================================================
		// Access the store directly (it is *repo.Memory which implements AuditRepo).
		auditRows, err := store.QueryAudit(ctx, repo.AuditFilter{IdentityID: userAID, Limit: 200})
		t.AssertNil(err)

		var foundCreated, foundRevoked bool
		for _, row := range auditRows {
			if row.Event == "pat.created" && row.ActorID == userAID && row.TargetID == userAID {
				foundCreated = true
			}
			if row.Event == "pat.revoked" && row.ActorID == userAID && row.TargetID == userAID {
				foundRevoked = true
			}
		}
		t.Assert(foundCreated, true)
		t.Assert(foundRevoked, true)
	})
}
