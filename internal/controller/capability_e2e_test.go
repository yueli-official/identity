package controller_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"

	"github.com/yueli-official/foundation/go/capability"
	"github.com/yueli-official/identity/internal/controller"
	"github.com/yueli-official/identity/internal/identitycap"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/repo"
	identityruntime "github.com/yueli-official/identity/internal/runtime"
)

type capabilityAuditStore struct {
	repo.AuditRepo
	fail atomic.Bool
}

func (store *capabilityAuditStore) InsertAudit(ctx context.Context, row repo.AuditRow) error {
	if store.fail.Load() {
		return errors.New("audit unavailable")
	}
	return store.AuditRepo.InsertAudit(ctx, row)
}

func TestCapabilityEndpointsRequireAdminAndAuditProbes(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		store := repo.NewMemory()
		auditStore := &capabilityAuditStore{AuditRepo: store}
		svc := logic.New(store, logic.DefaultConfig())
		auth := controller.New(svc, false)
		registry, err := identitycap.New(identitycap.Registration{
			Key: "dev-mail", Adapter: "dev", Registered: true, Enabled: true,
			CapabilityKeys: []string{"identity.reset-password", "identity.verify-email"}, Operations: []string{"send"},
			Checker: identitycap.HealthCheckFunc(func(context.Context) error { return nil }),
		})
		t.AssertNil(err)
		capCtl := controller.NewCapability(auth, registry, auditStore, capability.ServiceMetadata{Name: "identity", Version: "test", BuildSHA: "test", Deployment: "identity-test"})

		s := g.Server(t.Name())
		s.SetAddr("127.0.0.1:0")
		s.Use(controller.ActorMiddleware, identityruntime.APIMiddleware)
		s.Group("/", func(group *ghttp.RouterGroup) {
			group.Bind(auth)
			group.Bind(capCtl)
		})
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()
		base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())

		request := func(method, path string, headers map[string]string) (string, int) {
			req, requestErr := http.NewRequestWithContext(ctx, method, base+path, nil)
			t.AssertNil(requestErr)
			for key, value := range headers {
				req.Header.Set(key, value)
			}
			response, requestErr := http.DefaultClient.Do(req)
			t.AssertNil(requestErr)
			defer response.Body.Close()
			body := new(strings.Builder)
			_, _ = io.Copy(body, response.Body)
			return body.String(), response.StatusCode
		}
		newUser := func(email string) (string, map[string]string) {
			identity, createErr := svc.Register(ctx, logic.RegisterInput{Email: email, Password: "correct horse battery", DisplayName: email})
			t.AssertNil(createErr)
			login, loginErr := svc.Login(ctx, logic.LoginInput{Email: email, Password: "correct horse battery"})
			t.AssertNil(loginErr)
			return identity.ID, map[string]string{"Cookie": "id_session=" + login.SessionID}
		}
		adminID, adminHeaders := newUser("cap-admin@example.com")
		adminHeaders["X-Request-Id"] = "capability-test-request"
		t.AssertNil(svc.GrantRole(ctx, adminID, logic.AdminRole))
		_, userHeaders := newUser("cap-user@example.com")

		body, status := request(http.MethodGet, "/api/v1/admin/capabilities", nil)
		t.Assert(status, http.StatusUnauthorized)
		t.Assert(gjson.New(body).Get("code").String(), "identity.not_authenticated")
		body, status = request(http.MethodGet, "/api/v1/admin/capabilities", userHeaders)
		t.Assert(status, http.StatusForbidden)
		body, status = request(http.MethodGet, "/api/v1/admin/capabilities", adminHeaders)
		t.Assert(status, http.StatusOK)
		t.Assert(gjson.New(body).Get("manifest.service.name").String(), "identity")

		auditStore.fail.Store(true)
		body, status = request(http.MethodPost, "/api/v1/admin/providers/dev-mail/health-check", adminHeaders)
		t.Assert(status, http.StatusInternalServerError)
		t.Assert(gjson.New(body).Get("code").String(), "identity.capability_audit_unavailable")
		auditStore.fail.Store(false)
		body, status = request(http.MethodPost, "/api/v1/admin/providers/dev-mail/health-check", adminHeaders)
		t.Assert(status, http.StatusOK)
		t.Assert(gjson.New(body).Get("provider.health").String(), "healthy")
		rows, queryErr := store.QueryAudit(ctx, repo.AuditFilter{Event: "capability.provider_health_check", Limit: 10})
		t.AssertNil(queryErr)
		t.Assert(len(rows), 1)
		t.Assert(rows[0].ActorID, adminID)
		t.Assert(rows[0].Detail["provider"], "dev-mail")
		t.Assert(rows[0].RequestID, "capability-test-request")
		t.AssertNE(rows[0].IP, "")
		t.AssertNE(rows[0].UserAgent, "")

		// The sixth probe in one minute is rejected before another probe/audit.
		for range 3 {
			_, status = request(http.MethodPost, "/api/v1/admin/providers/dev-mail/health-check", adminHeaders)
			t.Assert(status, http.StatusOK)
		}
		body, status = request(http.MethodPost, "/api/v1/admin/providers/dev-mail/health-check", adminHeaders)
		t.Assert(status, http.StatusTooManyRequests)
		t.Assert(gjson.New(body).Get("code").String(), "identity.capability_probe_rate_limited")
	})
}
