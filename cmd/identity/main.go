// Command identity is the user-center IdP backend: an OIDC / OAuth2
// authorization server layered on the identity core.
package main

import (
	"errors"
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
	"platform/services/identity/internal/mailer"
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
	accountBaseURL := g.Cfg().MustGet(ctx, "account.baseUrl", "http://localhost:3000").String()

	// ── Data layer ──────────────────────────────────────────────────────────
	daoPG := dao.NewPG(g.DB())
	rdb := cache.NewRedis(g.Redis())

	// ── Business auth ───────────────────────────────────────────────────────
	store := repo.NewComposite(daoPG, rdb, rdb)
	cfg := logic.DefaultConfig()
	cfg.AccountBaseURL = accountBaseURL // base for verify-email / reset links

	// PAT HMAC secret: warn at startup when falling back to the insecure dev key.
	cfg.PATHMACSecret = g.Cfg().MustGet(ctx, "pat.hmacSecret").String()
	if cfg.PATHMACSecret == "" {
		g.Log().Warning(ctx, "pat.hmacSecret not set; using insecure dev fallback key")
	}
	if maxPerUser := g.Cfg().MustGet(ctx, "pat.maxPerUser", 0).Int(); maxPerUser > 0 {
		cfg.PATMaxPerUser = maxPerUser
	}

	svc := logic.New(store, cfg)
	authCtl := controller.New(svc, secureCookie)

	// ── Bootstrap admin (RBAC) ───────────────────────────────────────────────
	// If rbac.bootstrapAdminEmail is set and that identity already exists, grant
	// it the admin role. Best-effort + idempotent (GrantRole is a no-op on a
	// repeat grant); silent if the identity hasn't registered yet.
	if bootstrapEmail := g.Cfg().MustGet(ctx, "rbac.bootstrapAdminEmail").String(); bootstrapEmail != "" {
		id, err := svc.GetByEmail(ctx, bootstrapEmail)
		switch {
		case errors.Is(err, repo.ErrIdentityMissing):
			g.Log().Infof(ctx, "bootstrap admin: identity %q not found yet, skipping", bootstrapEmail)
		case err != nil:
			g.Log().Warningf(ctx, "bootstrap admin: lookup of %q failed: %v", bootstrapEmail, err)
		default:
			if err := svc.GrantRole(ctx, id.ID, logic.AdminRole); err != nil {
				g.Log().Warningf(ctx, "bootstrap admin: grant to %q failed: %v", bootstrapEmail, err)
			} else {
				g.Log().Infof(ctx, "bootstrap admin: granted %q to %q", logic.AdminRole, bootstrapEmail)
			}
		}
	}

	// ── Mailer ──────────────────────────────────────────────────────────────
	// Default to the dev-log mailer (links printed to logs); switch to real SMTP
	// only when mail.smtp.host is configured.
	var mlr mailer.Mailer = mailer.NewDev()
	if host := g.Cfg().MustGet(ctx, "mail.smtp.host").String(); host != "" {
		mlr = mailer.NewSMTP(
			host,
			g.Cfg().MustGet(ctx, "mail.smtp.port", "465").String(),
			g.Cfg().MustGet(ctx, "mail.smtp.username").String(),
			g.Cfg().MustGet(ctx, "mail.smtp.password").String(),
			g.Cfg().MustGet(ctx, "mail.from").String(),
			g.Cfg().MustGet(ctx, "mail.fromName").String(),
		)
	}
	svc.SetMailer(mlr)

	// ── OIDC / OAuth2 ───────────────────────────────────────────────────────
	mgr, err := oidc.NewManager(ctx, daoPG)
	if err != nil {
		panic(fmt.Sprintf("oidc.NewManager: %v", err))
	}
	// Durable PG-backed OIDC store: persists OAuth requests and refresh tokens
	// across restarts. Access tokens remain stateless JWTs.
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

	// ── Google OAuth login ──────────────────────────────────────────────────
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

	// Actor middleware runs globally (before all handlers) so that every
	// request — business API, OIDC endpoints, and OAuth login callbacks —
	// has IP / User-Agent / X-Request-Id available via actor.From(ctx).
	s.Use(controller.ActorMiddleware)

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
