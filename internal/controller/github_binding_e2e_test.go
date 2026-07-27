package controller_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/yueli-official/identity/internal/githubbinding"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/repo"
	identityruntime "github.com/yueli-official/identity/internal/runtime"
)

type githubBindingProvider struct {
	account githubbinding.Account
}

func (provider *githubBindingProvider) AuthorizationURL(state, challenge string) string {
	query := url.Values{"state": {state}, "code_challenge": {challenge}}
	return "https://github.test/authorize?" + query.Encode()
}

func (*githubBindingProvider) ExchangeCode(context.Context, string, string) (string, error) {
	return "ghu_test", nil
}

func (provider *githubBindingProvider) AuthenticatedUser(
	context.Context,
	string,
) (githubbinding.Account, error) {
	return provider.account, nil
}

func (*githubBindingProvider) RevokeAccessToken(context.Context, string) error {
	return nil
}

func TestGitHubBindingHTTPJourneyAndRevocationWebhook(t *testing.T) {
	identityStore := repo.NewMemory()
	service := logic.New(identityStore, logic.DefaultConfig())
	bindingStore := githubbinding.NewMemoryStore()
	provider := &githubBindingProvider{account: githubbinding.Account{
		AccountID: "24680", NodeID: "U_24680", Login: "publisher-login",
	}}
	module, err := githubbinding.New(githubbinding.Config{
		Store: bindingStore, Provider: provider,
		CipherSecret: []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatal(err)
	}
	const webhookSecret = "github-webhook-test-secret"
	githubController := controller.NewGitHubBinding(
		service, module, []byte(webhookSecret), "/",
	)
	baseController := controller.New(service, false)
	server := g.Server(t.Name())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(identityruntime.APIMiddleware)
		group.Bind(baseController)
		group.Bind(githubController)
	})
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.GET("/api/v1/account/github-bindings/callback", githubController.GitHubCallback)
		group.POST("/api/v1/webhooks/github", githubController.GitHubWebhook)
	})
	server.Start()
	defer server.Shutdown()
	base := fmt.Sprintf("http://127.0.0.1:%d", server.GetListenedPort())

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	postJSON(t, client, base+"/api/v1/auth/register", map[string]any{
		"email":       "github-binding@example.test",
		"password":    "Copper clouds cross the harbor 83!",
		"displayName": "GitHub Publisher",
	})
	postJSON(t, client, base+"/api/v1/auth/login", map[string]any{
		"email":    "github-binding@example.test",
		"password": "Copper clouds cross the harbor 83!",
	})

	startedBody := postJSON(
		t, client, base+"/api/v1/account/github-bindings/authorization",
		map[string]any{"returnTo": "/"},
	)
	started := gjson.New(startedBody)
	authorizationURL, err := url.Parse(started.Get("authorizationUrl").String())
	if err != nil || authorizationURL.Query().Get("state") == "" ||
		authorizationURL.Query().Get("code_challenge") == "" {
		t.Fatalf("begin response = %s", startedBody)
	}
	callback := base + "/api/v1/account/github-bindings/callback?" +
		url.Values{
			"state": {authorizationURL.Query().Get("state")},
			"code":  {"github-code"},
		}.Encode()
	response, err := client.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	bindings := getEnvelope(t, client, base+"/api/v1/account/github-bindings")
	if bindings.Get("bindings.0.providerAccountId").String() != "24680" ||
		bindings.Get("bindings.0.status").String() != githubbinding.StatusActive {
		t.Fatalf("bindings = %s", bindings.MustToJsonString())
	}

	payload := `{"action":"revoked","sender":{"id":24680,"login":"publisher-renamed"}}`
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	_, _ = mac.Write([]byte(payload))
	request, _ := http.NewRequest(
		http.MethodPost, base+"/api/v1/webhooks/github", strings.NewReader(payload),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Event", "github_app_authorization")
	request.Header.Set("X-GitHub-Delivery", "delivery-1")
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	webhookResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer webhookResponse.Body.Close()
	if webhookResponse.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(webhookResponse.Body)
		t.Fatalf("webhook status=%d body=%s", webhookResponse.StatusCode, body)
	}
	history := getEnvelope(t, client, base+"/api/v1/account/github-bindings")
	if history.Get("bindings.0.status").String() != githubbinding.StatusBlocked ||
		history.Get("bindings.0.login").String() != "publisher-renamed" {
		t.Fatalf("history after revoke = %s", history.MustToJsonString())
	}
}
