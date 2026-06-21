// Command identity-demo runs the IdP backend with fully IN-MEMORY storage — no
// Postgres, no Redis, no Docker. It exists so the account-center UI (apps/account)
// can be driven end-to-end locally with one command: register, log in, view the
// profile, log out, and exercise the OIDC authorize/token flow all work against
// volatile in-memory state that resets on restart.
//
// This is the same wiring the hermetic e2e tests use (repo.NewMemory +
// oidc.NewMemBackend), productized as a long-running server. DO NOT use it in
// production — state is lost on restart and the secrets below are public.
//
//	cd platform/services/identity && go run ./cmd/identity-demo
//	cd platform/apps/account && pnpm dev   # http://localhost:3000 (proxies /api → :8081)
package main

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
	"golang.org/x/crypto/bcrypt"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/mailer"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oauthlogin"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

const (
	// Fixed dev values. The issuer/listen port matches apps/account's devProxy
	// target (nuxt.config.ts → nitro.devProxy → http://localhost:8081).
	demoAddr     = "127.0.0.1:8081" // loopback: no Windows Firewall prompt
	demoIssuer   = "http://localhost:8081"
	demoLoginURL = "http://localhost:3000/login"
	demoSecret   = "demo-only-insecure-global-secret-0123456789" // >= 32 bytes (fosite)

	demoEmail    = "demo@example.com"
	demoPassword = "demo-password-123"
	demoClientID = "demo-web" // a sample consumer-site OIDC client (for SSO testing)

	// Confidential service client for the resource site's AssetClient: mints a
	// client_credentials service token (scope asset:sign) so the resource service
	// can sign GET URLs for login-gated downloads. The secret
	// must match resource's config (resource.assetService.serviceToken.clientSecret).
	demoResourceSvcID     = "resource-svc"
	demoResourceSvcSecret = "demo-resource-svc-secret-0123456789"
)

func main() {
	ctx := gctx.New()
	loadDotEnv() // pick up gitignored creds (Google client id/secret) from .env.local

	// ── In-memory data layer (resets on restart) ─────────────────────────────
	store := repo.NewMemory()
	cfg := logic.DefaultConfig()
	cfg.AccountBaseURL = "http://localhost:3000"
	svc := logic.New(store, cfg)
	svc.SetMailer(mailer.NewDev()) // verify-email / reset links are printed to the log

	// Seed a ready-to-use account and a sample OIDC client so the UI is usable
	// the moment both servers are up.
	if _, err := svc.Register(ctx, logic.RegisterInput{
		Email: demoEmail, Password: demoPassword, DisplayName: "Demo User",
	}); err != nil {
		g.Log().Warningf(ctx, "seed demo user: %v", err)
	} else if id, err := svc.GetByEmail(ctx, demoEmail); err == nil {
		// Demo convenience: pre-verify the seeded account so the UI isn't gated
		// behind an email link that the dev mailer only prints to this log.
		_ = store.SetEmailVerified(ctx, id.ID, true)
	}
	store.SetClient(model.OIDCClient{
		ID:            demoClientID,
		Public:        true,
		RedirectURIs:  []string{"http://localhost:3000/callback"},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile", "email", "roles", "offline_access"},
	})
	// Consumer-site OIDC client for the resource app's BFF.
	// Public + PKCE in dev; a confidential client (secret) is the prod hardening.
	store.SetClient(model.OIDCClient{
		ID:            "resource-web",
		Public:        true,
		RedirectURIs:  []string{"http://localhost:3001/auth/callback"},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile", "email", "roles", "offline_access"},
	})
	// Consumer-site OIDC client for the blog app's BFF (apps/blog :3002).
	store.SetClient(model.OIDCClient{
		ID:            "blog-web",
		Public:        true,
		RedirectURIs:  []string{"http://localhost:3002/auth/callback"},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		Scopes:        []string{"openid", "profile", "email", "roles", "offline_access"},
	})
	// Confidential service client (client_credentials) for the resource site's
	// AssetClient → mints a service token scoped `asset:sign` to sign GET URLs for
	// login-gated downloads. Secret is bcrypt-hashed; fosite
	// validates the presented secret against it on the token endpoint.
	svcSecretHash, _ := bcrypt.GenerateFromPassword([]byte(demoResourceSvcSecret), bcrypt.DefaultCost)
	store.SetClient(model.OIDCClient{
		ID:         demoResourceSvcID,
		Public:     false,
		SecretHash: string(svcSecretHash),
		GrantTypes: []string{"client_credentials"},
		Scopes:     []string{"asset:sign"},
	})

	authCtl := controller.New(svc, false) // insecure cookie: round-trips over plain HTTP

	// ── OIDC / OAuth2 (in-memory backend) ────────────────────────────────────
	mgr, err := oidc.NewManager(ctx, store)
	if err != nil {
		panic(err)
	}
	oidcStore := oidc.NewStore(oidc.NewMemBackend(), store)
	svc.SetRefreshRevoker(oidcStore)
	provider := oidc.NewProvider(oidcStore, oidc.Config{
		Issuer:       demoIssuer,
		GlobalSecret: []byte(demoSecret),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
		RefreshTTL:   720 * time.Hour,
	}, mgr.KeyGetter)
	oidcCtl := controller.NewOIDC(provider, mgr, svc, demoIssuer, demoLoginURL)
	// Consumer sites' RP-initiated logout bounces back here (end_session); allow
	// their dev origins (resource :3001, account :3000).
	oidcCtl.SetPostLogoutRedirects([]string{"http://localhost:3001", "http://localhost:3002", "http://localhost:3000"})

	// Google OAuth: enabled when GF_OIDC_GOOGLE_CLIENTID/SECRET are present
	// (read from .env.local via loadDotEnv). Otherwise nil → the start/callback
	// endpoints redirect to login with ?error=oauth_unavailable.
	var googleProvider oauthlogin.Provider
	googleID := os.Getenv("GF_OIDC_GOOGLE_CLIENTID")
	googleSecret := os.Getenv("GF_OIDC_GOOGLE_CLIENTSECRET")
	googleRedirect := os.Getenv("GF_OIDC_GOOGLE_REDIRECTURL")
	if googleRedirect == "" {
		googleRedirect = "http://localhost:3000/api/v1/auth/oauth/google/callback"
	}
	if googleID != "" && googleSecret != "" {
		googleProvider = oauthlogin.NewGoogle(googleID, googleSecret, googleRedirect)
		g.Log().Infof(ctx, "google oauth: ENABLED (redirect %s)", googleRedirect)
	} else {
		g.Log().Info(ctx, "google oauth: disabled (set GF_OIDC_GOOGLE_CLIENTID/SECRET in .env.local)")
	}
	oauthCtl := controller.NewOAuth(svc, googleProvider, false, []byte(demoSecret), demoLoginURL)

	// ── Routing (mirrors cmd/identity) ───────────────────────────────────────
	s := g.Server()
	s.SetAddr(demoAddr)
	s.SetDumpRouterMap(false)
	s.Use(controller.ActorMiddleware)

	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(ghttpx.Middleware)
		grp.GET("/healthz", controller.Healthz)
		grp.Bind(authCtl)
	})
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/.well-known/openid-configuration", oidcCtl.Discovery)
		grp.GET("/oauth2/jwks.json", oidcCtl.JWKS)
		grp.GET("/oauth2/authorize", oidcCtl.Authorize)
		grp.POST("/oauth2/token", oidcCtl.Token)
		grp.ALL("/oauth2/userinfo", oidcCtl.Userinfo)
		grp.POST("/oauth2/revoke", oidcCtl.Revoke)
		grp.ALL("/oauth2/end_session", oidcCtl.EndSession)
	})
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/api/v1/auth/oauth/google/start", oauthCtl.GoogleStart)
		grp.GET("/api/v1/auth/oauth/google/callback", oauthCtl.GoogleCallback)
	})

	g.Log().Info(ctx, "── identity-demo (IN-MEMORY, state resets on restart) ──")
	g.Log().Infof(ctx, "backend:  %s   (account app proxies /api here)", demoIssuer)
	g.Log().Infof(ctx, "UI:       http://localhost:3000")
	g.Log().Infof(ctx, "seeded login:  %s / %s", demoEmail, demoPassword)
	s.Run()
}

// loadDotEnv loads KEY=VALUE lines from .env.local in the working directory into
// the process environment (existing env wins). Dev-only convenience so `go run`
// picks up the gitignored credentials file the same way the real service expects
// those values to be present in the environment. Missing file → no-op.
func loadDotEnv() {
	f, err := os.Open(".env.local")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}
