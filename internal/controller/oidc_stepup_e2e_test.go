package controller_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/google/uuid"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/controller"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/oidc"
	"github.com/yueli-official/identity/internal/repo"
)

func TestE2E_OIDCAssuranceStepUpGate(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		const clientID = "step-up-web"
		ctx := context.Background()
		store := repo.NewMemory()
		store.SetClient(model.OIDCClient{
			ID: clientID, Public: true,
			RedirectURIs:  []string{rbacCallbackURI},
			GrantTypes:    []string{"authorization_code"},
			ResponseTypes: []string{"code"},
			Scopes:        []string{"openid", "profile"},
		})
		service := logic.New(store, logic.DefaultConfig())
		registered, err := service.Register(ctx, logic.RegisterInput{
			Email: "oidc-step-up@example.com", Password: "correct horse battery staple",
		})
		t.AssertNil(err)
		login, err := service.Login(ctx, logic.LoginInput{
			Email: registered.Email, Password: "correct horse battery staple",
		})
		t.AssertNil(err)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		t.AssertNil(err)
		port := listener.Addr().(*net.TCPAddr).Port
		t.AssertNil(listener.Close())
		base := fmt.Sprintf("http://127.0.0.1:%d", port)

		manager, err := oidc.NewManager(ctx, store)
		t.AssertNil(err)
		provider := oidc.NewProvider(
			oidc.NewStore(oidc.NewMemBackend(), store),
			oidc.Config{
				Issuer: base, GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
				AccessTTL: 10 * time.Minute, IDTTL: 10 * time.Minute,
			},
			manager.KeyGetter,
		)
		oidcController := controller.NewOIDC(
			provider, manager, service, store, base, base+"/login", false,
			[]byte("0123456789abcdef0123456789abcdef"),
		)
		server := ghttp.GetServer(t.Name())
		server.SetAddr(fmt.Sprintf("127.0.0.1:%d", port))
		server.SetDumpRouterMap(false)
		server.BindHandler("GET:/oauth2/authorize", oidcController.Authorize)
		server.Start()
		defer server.Shutdown()

		passwordOnly := authorizeForAssurance(
			t, ctx, base, clientID, login.SessionID,
			string(authentication.ProfileMultiFactor), "",
		)
		t.Assert(strings.HasPrefix(passwordOnly.String(), base+"/login"), true)
		returnTo, err := url.Parse(passwordOnly.Query().Get("return_to"))
		t.AssertNil(err)
		t.Assert(returnTo.Query().Get("acr_values"), string(authentication.ProfileMultiFactor))

		silent := authorizeForAssurance(
			t, ctx, base, clientID, login.SessionID,
			string(authentication.ProfileMultiFactor), "none",
		)
		t.Assert(silent.Host+silent.Path, "127.0.0.1/callback")
		t.Assert(silent.Query().Get("error"), "login_required")

		session, err := store.GetSession(ctx, login.SessionID)
		t.AssertNil(err)
		session.Authentication = authentication.MultiFactor(
			session.Authentication, uuid.NewString(), time.Now().UTC(), "totp-test",
		)
		t.AssertNil(store.CreateSession(ctx, session, time.Hour))

		elevated := authorizeForAssurance(
			t, ctx, base, clientID, login.SessionID,
			string(authentication.ProfileMultiFactor), "",
		)
		t.Assert(elevated.Host+elevated.Path, "127.0.0.1/callback")
		if elevated.Query().Get("code") == "" {
			t.Fatalf("elevated authorize did not issue a code: %s", elevated)
		}
	})
}

func authorizeForAssurance(
	t *gtest.T,
	ctx context.Context,
	base string,
	clientID string,
	sessionID string,
	acr string,
	prompt string,
) *url.URL {
	verifier := strings.Repeat("a", 48)
	digest := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {rbacCallbackURI},
		"scope":                 {"openid profile"},
		"state":                 {"oidc-step-up-state"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(digest[:])},
		"code_challenge_method": {"S256"},
		"acr_values":            {acr},
	}
	if prompt != "" {
		query.Set("prompt", prompt)
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, base+"/oauth2/authorize?"+query.Encode(), nil,
	)
	t.AssertNil(err)
	request.Header.Set("Cookie", "id_session="+sessionID)
	response, err := (&http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}).Do(request)
	t.AssertNil(err)
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound && response.StatusCode != http.StatusSeeOther {
		t.Fatalf("authorize status = %d, want redirect", response.StatusCode)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	t.AssertNil(err)
	return location
}
