// Command identity is the user-center IdP backend (milestone 3: OIDC /
// OAuth2 authorization server layered on top of the milestone 2 identity core).
package main

import (
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/cache"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oauthlogin"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

func main() {
	ctx := gctx.New()

	// ── Config ──────────────────────────────────────────────────────────────
	issuer := g.Cfg().MustGet(ctx, "oidc.issuer").String()
	loginURL := g.Cfg().MustGet(ctx, "account.loginUrl").String()
	globalSecret := g.Cfg().MustGet(ctx, "oidc.globalSecret").String()
	if len(globalSecret) < 32 {
		panic(fmt.Sprintf("oidc.globalSecret must be >= 32 bytes (got %d); set GF_OIDC_GLOBALSECRET", len(globalSecret)))
	}
	secureCookie := g.Cfg().MustGet(ctx, "cookie.secure", true).Bool()

	// ── Data layer ──────────────────────────────────────────────────────────
	daoPG := dao.NewPG(g.DB())
	rdb := cache.NewRedis(g.Redis())

	// ── Business auth (milestone ②) ─────────────────────────────────────────
	store := repo.NewComposite(daoPG, rdb, rdb)
	svc := logic.New(store, logic.DefaultConfig())
	authCtl := controller.New(svc, secureCookie)

	// ── OIDC / OAuth2 (milestone ③ + ④) ────────────────────────────────────
	mgr, err := oidc.NewManager(ctx, daoPG)
	if err != nil {
		panic(fmt.Sprintf("oidc.NewManager: %v", err))
	}
	// Durable PG-backed OIDC store: persists OAuth requests and refresh tokens
	// across restarts (milestone ④). Access tokens remain stateless JWTs.
	oidcStore := oidc.NewStore(oidc.NewPGBackend(g.DB()), daoPG)
	// Wire passive logout: logic.Service calls RevokeRefreshBySession on logout,
	// which revokes all refresh tokens belonging to that identity session.
	svc.SetRefreshRevoker(oidcStore)
	refreshTTL := g.Cfg().MustGet(ctx, "oidc.refreshTtl", "720h").Duration()
	provider := oidc.NewProvider(oidcStore, oidc.Config{
		Issuer:       issuer,
		GlobalSecret: []byte(globalSecret),
		AccessTTL:    10 * time.Minute,
		IDTTL:        10 * time.Minute,
		RefreshTTL:   refreshTTL,
	}, mgr.KeyGetter)
	oidcCtl := controller.NewOIDC(provider, mgr, svc, issuer, loginURL)

	// ── Google OAuth login (milestone ⑤) ────────────────────────────────────
	// Provider stays nil when credentials are unconfigured; the controller then
	// redirects the start/callback endpoints to login with ?error=oauth_unavailable.
	googleClientID := g.Cfg().MustGet(ctx, "oidc.google.clientId").String()
	googleClientSecret := g.Cfg().MustGet(ctx, "oidc.google.clientSecret").String()
	googleRedirectURL := g.Cfg().MustGet(ctx, "oidc.google.redirectUrl").String()
	var googleProvider oauthlogin.Provider
	if googleClientID != "" && googleClientSecret != "" {
		googleProvider = oauthlogin.NewGoogle(googleClientID, googleClientSecret, googleRedirectURL)
	}
	oauthCtl := controller.NewOAuth(svc, googleProvider, secureCookie, []byte(globalSecret), loginURL)

	// ── Routing ─────────────────────────────────────────────────────────────
	s := g.Server()

	// Business API: JSON envelope middleware applied to this group only.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.Middleware(ghttpx.Middleware)
		grp.GET("/healthz", controller.Healthz)
		grp.Bind(authCtl)
	})

	// OIDC standard endpoints: raw RFC responses — NO envelope middleware.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/.well-known/openid-configuration", oidcCtl.Discovery)
		grp.GET("/oauth2/jwks.json", oidcCtl.JWKS)
		grp.GET("/oauth2/authorize", oidcCtl.Authorize)
		grp.POST("/oauth2/token", oidcCtl.Token)
		grp.ALL("/oauth2/userinfo", oidcCtl.Userinfo)
		grp.POST("/oauth2/revoke", oidcCtl.Revoke)         // RFC 7009 token revocation
		grp.ALL("/oauth2/end_session", oidcCtl.EndSession)  // OIDC RP-initiated logout (MVP)
	})

	// Google OAuth login: raw redirect handlers — NO envelope middleware.
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/api/v1/auth/oauth/google/start", oauthCtl.GoogleStart)
		grp.GET("/api/v1/auth/oauth/google/callback", oauthCtl.GoogleCallback)
	})

	g.Log().Info(ctx, "identity-service starting")
	s.Run()
}
