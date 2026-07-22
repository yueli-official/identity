package controller_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	coreauth "github.com/yueli-official/foundation/go/auth"
	"github.com/yueli-official/foundation/go/problem"

	"platform/gokit/authhttp"
	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

type assetProxyVerifierFunc func(context.Context, string) (*coreauth.Principal, error)

func (function assetProxyVerifierFunc) Verify(ctx context.Context, raw string) (*coreauth.Principal, error) {
	return function(ctx, raw)
}

func TestAssetAdminProxyAcceptsVerifiedAdminBearerAndAdminCookie(t *testing.T) {
	ctx := context.Background()
	store := repo.NewMemory()
	service := logic.New(store, logic.DefaultConfig())
	baseController := controller.New(service, false)

	newIdentity := func(email string, admin bool) (string, string) {
		identity, err := service.Register(ctx, logic.RegisterInput{
			Email: email, Password: "longenough123", DisplayName: email,
		})
		if err != nil {
			t.Fatal(err)
		}
		if admin {
			if err := service.GrantRole(ctx, identity.ID, logic.AdminRole); err != nil {
				t.Fatal(err)
			}
		}
		login, err := service.Login(ctx, logic.LoginInput{Email: email, Password: "longenough123"})
		if err != nil {
			t.Fatal(err)
		}
		return identity.ID, login.SessionID
	}

	adminID, adminSession := newIdentity("asset-admin@example.test", true)
	userID, userSession := newIdentity("asset-user@example.test", false)

	manager, err := oidc.NewManager(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	const issuer = "https://identity.example.test"
	const audience = "asset-api"
	serviceTokenVerifier, err := coreauth.NewVerifier(coreauth.Config{
		Keys: manager, Issuer: issuer, Audiences: []string{audience},
	})
	if err != nil {
		t.Fatal(err)
	}

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		raw := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		principal, verifyErr := serviceTokenVerifier.Verify(request.Context(), raw)
		if verifyErr != nil {
			http.Error(writer, verifyErr.Error(), http.StatusUnauthorized)
			return
		}
		if principal.Subject != adminID || !principal.HasScope("asset:sign") {
			http.Error(writer, "unexpected delegated principal", http.StatusForbidden)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"items":[{"key":"docs-main"}]}`))
	}))
	defer upstream.Close()

	proxy := controller.NewAssetAdminProxy(baseController, manager, issuer, upstream.URL, audience)
	verifier := assetProxyVerifierFunc(func(_ context.Context, raw string) (*coreauth.Principal, error) {
		switch raw {
		case "admin-token":
			return &coreauth.Principal{Subject: adminID, Roles: []string{logic.AdminRole}}, nil
		case "user-token":
			return &coreauth.Principal{Subject: userID, Roles: []string{logic.DefaultRole}}, nil
		default:
			return nil, errors.New("invalid test token")
		}
	})

	server := g.Server(t.Name())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttpx.Middleware, authhttp.Optional(verifier))
		group.ALL("/api/v1/admin/assets-proxy/*", proxy.Forward)
	})
	server.Start()
	defer server.Shutdown()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", server.GetListenedPort())

	tests := []struct {
		name       string
		headers    map[string]string
		wantStatus int
		wantCode   string
		wantCalls  int32
	}{
		{name: "anonymous", wantStatus: http.StatusUnauthorized, wantCode: "identity.not_authenticated", wantCalls: 0},
		{name: "ordinary user cookie", headers: map[string]string{"Cookie": "id_session=" + userSession}, wantStatus: http.StatusForbidden, wantCode: "identity.forbidden", wantCalls: 0},
		{name: "non-admin bearer", headers: map[string]string{"Authorization": "Bearer user-token"}, wantStatus: http.StatusForbidden, wantCode: "identity.forbidden", wantCalls: 0},
		{name: "admin bearer", headers: map[string]string{"Authorization": "Bearer admin-token"}, wantStatus: http.StatusOK, wantCalls: 1},
		{name: "admin cookie", headers: map[string]string{"Cookie": "id_session=" + adminSession}, wantStatus: http.StatusOK, wantCalls: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstreamCalls.Store(0)
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/admin/assets-proxy/sites", nil)
			if err != nil {
				t.Fatal(err)
			}
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantCode != "" {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := problem.Decode(body)
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Code != test.wantCode {
					t.Fatalf("code = %q, want %q", decoded.Code, test.wantCode)
				}
			}
			if calls := upstreamCalls.Load(); calls != test.wantCalls {
				t.Fatalf("upstream calls = %d, want %d", calls, test.wantCalls)
			}
		})
	}
}
