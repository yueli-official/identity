package controller_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/yueli-official/identity/internal/controller"
	"github.com/yueli-official/identity/internal/externallogin"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/oauthlogin"
	"github.com/yueli-official/identity/internal/repo"
	identityruntime "github.com/yueli-official/identity/internal/runtime"
)

func TestExternalLoginProviderControlPlane(t *testing.T) {
	ctx := context.Background()
	store := repo.NewMemory()
	service := logic.New(store, logic.DefaultConfig())
	baseController := controller.New(service, false)
	admin, err := service.Register(ctx, logic.RegisterInput{
		Email: "provider-admin@example.test", Password: "correct horse battery", DisplayName: "Provider Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.GrantRole(ctx, admin.ID, logic.AdminRole); err != nil {
		t.Fatal(err)
	}
	login, err := service.Login(ctx, logic.LoginInput{
		Email: admin.Email, Password: "correct horse battery",
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := externallogin.New(
		externallogin.NewMemoryStore(),
		"0123456789abcdef0123456789abcdef",
		"https://account.example.test",
	)
	if err != nil {
		t.Fatal(err)
	}
	externalController := controller.NewExternalLoginController(baseController, manager)
	server := g.Server(t.Name())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(identityruntime.APIMiddleware)
		group.Bind(externalController)
	})
	server.Start()
	defer server.Shutdown()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", server.GetListenedPort())

	call := func(method, path, body, cookie string) (int, string) {
		request, requestErr := http.NewRequestWithContext(ctx, method, baseURL+path, strings.NewReader(body))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		response, responseErr := http.DefaultClient.Do(request)
		if responseErr != nil {
			t.Fatal(responseErr)
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(data)
	}

	if status, _ := call(http.MethodGet, "/api/v1/admin/login-providers", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated admin list status=%d", status)
	}
	adminCookie := "id_session=" + login.SessionID
	status, body := call(http.MethodPut, "/api/v1/admin/login-providers/qq", `{"clientId":"qq-app","clientSecret":"qq-secret","enabled":true}`, adminCookie)
	if status != http.StatusOK || strings.Contains(body, "qq-secret") || !strings.Contains(body, `"secretVersion":1`) {
		t.Fatalf("save status=%d body=%s", status, body)
	}
	status, publicBody := call(http.MethodGet, "/api/v1/auth/oauth/providers", "", "")
	if status != http.StatusOK || !strings.Contains(publicBody, `"key":"qq"`) || strings.Contains(publicBody, "clientId") {
		t.Fatalf("public status=%d body=%s", status, publicBody)
	}
	provider, policy, err := manager.Resolve(ctx, "qq")
	if err != nil || provider.Name() != "qq" || policy != oauthlogin.RegistrationExistingOnly {
		t.Fatalf("resolved provider=%v policy=%q err=%v", provider, policy, err)
	}
}
