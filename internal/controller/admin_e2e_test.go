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

// TestAdminRoleEndpoint is a hermetic e2e test for the admin-guarded role
// endpoints. It proves the security-critical property: the admin guard runs
// BEFORE any state change, so an unauthenticated or non-admin caller can never
// mutate roles. It also checks the HTTP status mapping for each failure mode.
func TestAdminRoleEndpoint(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		store := repo.NewMemory()
		svc := logic.New(store, logic.DefaultConfig())
		ctl := controller.New(svc, false)
		ctx := context.Background()

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

		do := func(method, path string, headers map[string]string) (string, int) {
			req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
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

		// helper: register an identity and log it in, returning (id, sessionCookieHeader).
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

		grantPath := "/api/v1/admin/identities/" + targetID + "/roles"

		// 1. Unauthenticated grant → 401 not_authenticated; target unchanged.
		{
			body, status := do(http.MethodPost, grantPath+"?role=admin", nil)
			t.Assert(status, 401)
			t.Assert(gjson.New(body).Get("code").String(), "identity.not_authenticated")
			roles, _ := svc.GetRoles(ctx, targetID)
			t.Assert(len(roles), 1) // still just the default "user" — no mutation
		}

		// 2. Non-admin grant → 403 forbidden; target still unchanged (guard before mutation).
		{
			body, status := do(http.MethodPost, grantPath+"?role=admin", userHdr)
			t.Assert(status, 403)
			t.Assert(gjson.New(body).Get("code").String(), "identity.forbidden")
			roles, _ := svc.GetRoles(ctx, targetID)
			t.Assert(len(roles), 1)
		}

		// 3. Admin grant of unknown role → 400 unknown_role; target unchanged.
		{
			body, status := do(http.MethodPost, grantPath+"?role=superuser", adminHdr)
			t.Assert(status, 400)
			t.Assert(gjson.New(body).Get("code").String(), "identity.unknown_role")
			roles, _ := svc.GetRoles(ctx, targetID)
			t.Assert(len(roles), 1)
		}

		// 4. Admin grant of admin → 200; target now has admin.
		{
			body, status := do(http.MethodPost, grantPath+"?role=admin", adminHdr)
			t.Assert(status, 200)
			j := gjson.New(body)
			t.Assert(j.Get("code").String(), "ok")
			roles, _ := svc.GetRoles(ctx, targetID)
			t.Assert(len(roles), 2)
		}

		// 5. Admin revoke of admin → 200; target back to just "user".
		{
			body, status := do(http.MethodDelete, grantPath+"/admin", adminHdr)
			t.Assert(status, 200)
			t.Assert(gjson.New(body).Get("code").String(), "ok")
			roles, _ := svc.GetRoles(ctx, targetID)
			t.Assert(len(roles), 1)
			t.Assert(roles[0], logic.DefaultRole)
		}
	})
}
