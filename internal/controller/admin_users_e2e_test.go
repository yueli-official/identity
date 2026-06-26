package controller_test

import (
	"context"
	"errors"
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
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// TestAdminUserManagement is a hermetic e2e test for the admin user-management
// endpoints (list / stats / status / delete / reset-password / create). It
// proves the admin guard gates every mutation, the self-lockout guard blocks an
// admin from disabling themselves, and that a ban/delete actually bars login.
func TestAdminUserManagement(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		store := repo.NewMemory()
		svc := logic.New(store, logic.DefaultConfig())
		ctl := controller.New(svc, false)
		ctx := context.Background()

		s := g.Server(t.Name())
		s.SetAddr("127.0.0.1:0")
		s.Use(ghttpx.Middleware)
		s.Group("/", func(grp *ghttp.RouterGroup) {
			grp.Bind(ctl)
		})
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

		do := func(method, path, body string, headers map[string]string) (string, int) {
			var rdr io.Reader
			if body != "" {
				rdr = strings.NewReader(body)
			}
			req, err := http.NewRequestWithContext(ctx, method, base+path, rdr)
			t.AssertNil(err)
			if body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
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

		mkUser := func(email string) (string, map[string]string) {
			id, err := svc.Register(ctx, logic.RegisterInput{
				Email: email, Password: "longenough123", DisplayName: email,
			})
			t.AssertNil(err)
			out, err := svc.Login(ctx, logic.LoginInput{Email: email, Password: "longenough123"})
			t.AssertNil(err)
			return id.ID, map[string]string{"Cookie": "id_session=" + out.SessionID}
		}

		adminID, adminHdr := mkUser("admin@b.com")
		t.AssertNil(svc.GrantRole(ctx, adminID, logic.AdminRole))
		_, userHdr := mkUser("user@b.com")
		targetID, _ := mkUser("target@b.com")

		// 1. Non-admin list → 403.
		{
			body, status := do(http.MethodGet, "/api/v1/admin/users", "", userHdr)
			t.Assert(status, 403)
			t.Assert(gjson.New(body).Get("code").String(), "identity.forbidden")
		}

		// 2. Admin list → 200; sees all 3 non-deleted users.
		{
			body, status := do(http.MethodGet, "/api/v1/admin/users", "", adminHdr)
			t.Assert(status, 200)
			j := gjson.New(body)
			t.Assert(j.Get("data.total").Int(), 3)
			t.Assert(len(j.Get("data.list").Array()), 3)
		}

		// 3. Admin stats → total 3, active 3.
		{
			body, status := do(http.MethodGet, "/api/v1/admin/users/stats", "", adminHdr)
			t.Assert(status, 200)
			j := gjson.New(body)
			t.Assert(j.Get("data.total").Int(), 3)
			t.Assert(j.Get("data.active").Int(), 3)
		}

		// 4. Keyword filter narrows to one.
		{
			body, status := do(http.MethodGet, "/api/v1/admin/users?keyword=target", "", adminHdr)
			t.Assert(status, 200)
			t.Assert(gjson.New(body).Get("data.total").Int(), 1)
		}

		// 5. Self-ban → 403 (self-lockout guard).
		{
			path := "/api/v1/admin/users/" + adminID + "/status"
			body, status := do(http.MethodPut, path, `{"status":"disabled"}`, adminHdr)
			t.Assert(status, 403)
			t.Assert(gjson.New(body).Get("code").String(), "identity.forbidden")
		}

		// 6. Invalid status value → 400.
		{
			path := "/api/v1/admin/users/" + targetID + "/status"
			body, status := do(http.MethodPut, path, `{"status":"nonsense"}`, adminHdr)
			t.Assert(status, 400)
			t.Assert(gjson.New(body).Get("code").String(), "identity.invalid_status")
		}

		// 7. Ban target → 200, status disabled; login now barred.
		{
			path := "/api/v1/admin/users/" + targetID + "/status"
			body, status := do(http.MethodPut, path, `{"status":"disabled"}`, adminHdr)
			t.Assert(status, 200)
			t.Assert(gjson.New(body).Get("data.user.status").String(), "disabled")

			_, err := svc.Login(ctx, logic.LoginInput{Email: "target@b.com", Password: "longenough123"})
			t.Assert(errors.Is(err, iderr.AccountDisabled()) || err != nil, true)
		}

		// 8. Stats reflect the ban: active 2, disabled 1.
		{
			body, _ := do(http.MethodGet, "/api/v1/admin/users/stats", "", adminHdr)
			j := gjson.New(body)
			t.Assert(j.Get("data.active").Int(), 2)
			t.Assert(j.Get("data.disabled").Int(), 1)
		}

		// 9. Admin reset target's password → 200; new password works after unban.
		{
			path := "/api/v1/admin/users/" + targetID + "/password"
			_, status := do(http.MethodPost, path, `{"newPassword":"brandnewpw99"}`, adminHdr)
			t.Assert(status, 200)
			// unban so login can proceed, then verify the new password.
			unban := "/api/v1/admin/users/" + targetID + "/status"
			_, st := do(http.MethodPut, unban, `{"status":"active"}`, adminHdr)
			t.Assert(st, 200)
			_, err := svc.Login(ctx, logic.LoginInput{Email: "target@b.com", Password: "brandnewpw99"})
			t.AssertNil(err)
		}

		// 10. Admin create user (with admin role) → 200; appears in list; can log in.
		{
			body, status := do(http.MethodPost, "/api/v1/admin/users",
				`{"email":"created@b.com","password":"createdpw123","displayName":"Created","roles":["admin"]}`, adminHdr)
			t.Assert(status, 200)
			j := gjson.New(body)
			t.Assert(j.Get("data.user.email").String(), "created@b.com")
			roles := j.Get("data.user.roles").Strings()
			t.Assert(contains(roles, "admin"), true)
			_, err := svc.Login(ctx, logic.LoginInput{Email: "created@b.com", Password: "createdpw123"})
			t.AssertNil(err)
		}

		// 11. Soft-delete target → 200; gone from default list, login barred.
		{
			path := "/api/v1/admin/users/" + targetID
			_, status := do(http.MethodDelete, path, "", adminHdr)
			t.Assert(status, 200)
			body, _ := do(http.MethodGet, "/api/v1/admin/users?keyword=target", "", adminHdr)
			t.Assert(gjson.New(body).Get("data.total").Int(), 0)
			_, err := svc.Login(ctx, logic.LoginInput{Email: "target@b.com", Password: "brandnewpw99"})
			t.AssertNE(err, nil)
		}
	})
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
