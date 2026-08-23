package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/oauthlogin"
)

// oauthStateCookie holds the short-lived CSRF nonce that must match the nonce
// signed into the OAuth state parameter at /callback.
const oauthStateCookiePrefix = "g_oauth_state_"

type OAuthProviderSource interface {
	Resolve(context.Context, string) (oauthlogin.Provider, oauthlogin.RegistrationPolicy, error)
}

type staticOAuthProviderSource struct {
	provider oauthlogin.Provider
}

func (source staticOAuthProviderSource) Resolve(_ context.Context, key string) (oauthlogin.Provider, oauthlogin.RegistrationPolicy, error) {
	if source.provider == nil || source.provider.Name() != key {
		return nil, "", errors.New("provider unavailable")
	}
	return source.provider, oauthlogin.RegistrationVerifiedEmail, nil
}

// OAuthController handles the external-provider (Google) login redirect dance.
// These are raw redirect endpoints that bypass the gokit JSON envelope, the same
// precedent as the OIDC endpoints.
type OAuthController struct {
	svc          *logic.Service
	providers    OAuthProviderSource
	secureCookie bool
	sessionTTL   time.Duration
	stateSecret  []byte
	loginURL     string
}

// NewOAuth builds the OAuth controller. google may be nil when no credentials are
// configured, in which case the endpoints redirect to the login page with an error.
func NewOAuth(svc *logic.Service, google oauthlogin.Provider, secureCookie bool, stateSecret []byte, loginURL string, sessionTTL ...time.Duration) *OAuthController {
	return NewOAuthRegistry(svc, staticOAuthProviderSource{provider: google}, secureCookie, stateSecret, loginURL, sessionTTL...)
}

func NewOAuthRegistry(svc *logic.Service, providers OAuthProviderSource, secureCookie bool, stateSecret []byte, loginURL string, sessionTTL ...time.Duration) *OAuthController {
	ttl := logic.DefaultConfig().SessionIdleTTL
	if len(sessionTTL) > 0 {
		ttl = sessionTTL[0]
	}
	return &OAuthController{svc: svc, providers: providers, secureCookie: secureCookie, sessionTTL: ttl, stateSecret: stateSecret, loginURL: loginURL}
}

// GoogleStart handles GET /api/v1/auth/oauth/google/start.
// It signs the (validated) return_to + a random nonce into the OAuth state,
// drops the nonce in a short-lived HttpOnly cookie, then redirects to Google.
func (c *OAuthController) GoogleStart(r *ghttp.Request) {
	c.start(r, "google")
}

func (c *OAuthController) GoogleCallback(r *ghttp.Request) {
	c.callback(r, "google")
}

func (c *OAuthController) QQStart(r *ghttp.Request) {
	c.start(r, "qq")
}

func (c *OAuthController) QQCallback(r *ghttp.Request) {
	c.callback(r, "qq")
}

func (c *OAuthController) Start(r *ghttp.Request) {
	c.start(r, strings.ToLower(strings.TrimSpace(r.Get("provider").String())))
}

func (c *OAuthController) Callback(r *ghttp.Request) {
	c.callback(r, strings.ToLower(strings.TrimSpace(r.Get("provider").String())))
}

func (c *OAuthController) start(r *ghttp.Request, key string) {
	provider, _, err := c.providers.Resolve(r.Context(), key)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_unavailable")
		return
	}
	returnTo := safeReturnTo(r.Get("return_to").String())
	// intent=bind + a live session → LINK the provider to the current identity
	// (account binding) instead of logging in. The identity id is signed into the
	// state so the callback can trust it.
	bind := ""
	if r.Get("intent").String() == "bind" {
		if id, err := c.svc.Me(r.Context(), r.Cookie.Get(sessionCookie, "").String()); err == nil {
			bind = id.ID
		}
	}
	nonce := randHex(16)
	state := oauthlogin.EncodeProviderState(c.stateSecret, key, returnTo, nonce, bind, time.Now().Add(10*time.Minute).Unix())
	r.Cookie.SetHttpCookie(&http.Cookie{
		Name: oauthStateCookiePrefix + key, Value: nonce, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	r.Response.RedirectTo(provider.AuthorizeURL(state))
}

// GoogleCallback handles GET /api/v1/auth/oauth/google/callback.
// It verifies the signed state + nonce cookie (CSRF), exchanges the code, fetches
// the profile, performs account-linking via OAuthLogin, mints an id_session
// cookie, and redirects back to the validated return_to.
func (c *OAuthController) callback(r *ghttp.Request, key string) {
	ctx := r.Context()
	provider, policy, err := c.providers.Resolve(ctx, key)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_unavailable")
		return
	}
	stateProvider, returnTo, nonce, bind, err := oauthlogin.DecodeProviderState(c.stateSecret, r.Get("state").String())
	cookieName := oauthStateCookiePrefix + key
	cookieNonce := r.Cookie.Get(cookieName, "").String()
	r.Cookie.Remove(cookieName)
	if err != nil || stateProvider != key || nonce == "" || nonce != cookieNonce {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_state")
		return
	}
	code := r.Get("code").String()
	if code == "" {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_denied")
		return
	}
	at, err := provider.ExchangeCode(ctx, code)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_exchange")
		return
	}
	ui, err := provider.FetchUserInfo(ctx, at)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_userinfo")
		return
	}

	// Bind flow: link the provider to the already-logged-in identity (no new
	// session) and return to the account page. returnTo is the same-origin page
	// the account UI started the bind from.
	if bind != "" {
		current, sessionErr := c.svc.Me(
			ctx,
			r.Cookie.Get(sessionCookie, "").String(),
		)
		if sessionErr != nil || current.ID != bind {
			r.Response.RedirectTo(withProviderError(returnTo, "oauth_bind", key))
			return
		}
		if berr := c.svc.BindOAuth(ctx, bind, provider.Name(), ui.ProviderUID, ui.Email, ui.EmailVerified); berr != nil {
			r.Response.RedirectTo(withProviderError(returnTo, "oauth_bind", key))
			return
		}
		r.Response.RedirectTo(returnTo)
		return
	}

	out, err := c.svc.OAuthLogin(ctx, logic.OAuthLoginInput{
		Provider: provider.Name(), ProviderUID: ui.ProviderUID, Email: ui.Email,
		EmailVerified: ui.EmailVerified, DisplayName: ui.DisplayName,
		UserAgent: r.UserAgent(), IP: r.GetClientIp(),
		RegistrationPolicy: policy,
	})
	if err != nil {
		code := "oauth_login"
		if failure, ok := iderr.Resolve(err); ok {
			switch failure.Code {
			case iderr.CodeOAuthEmailConflict:
				code = "oauth_link_required"
			case iderr.CodeOAuthBindingRequired:
				code = "oauth_binding_required"
			}
		}
		r.Response.RedirectTo(c.loginURL + "?error=" + code + "&provider=" + url.QueryEscape(key))
		return
	}
	if out.MFARequired {
		target, parseErr := url.Parse(c.loginURL)
		if parseErr != nil {
			r.Response.RedirectTo(c.loginURL + "?error=oauth_login")
			return
		}
		query := target.Query()
		query.Set("mfa_transaction", out.MFATransaction)
		query.Set("return_to", returnTo)
		target.RawQuery = query.Encode()
		r.Response.RedirectTo(target.String())
		return
	}
	cookie := &http.Cookie{
		Name: sessionCookie, Value: out.SessionID, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode,
	}
	if c.sessionTTL > 0 {
		cookie.MaxAge = int(c.sessionTTL.Seconds())
		cookie.Expires = time.Now().Add(c.sessionTTL)
	}
	r.Cookie.SetHttpCookie(cookie)
	r.Response.RedirectTo(returnTo)
}

// safeReturnTo permits only same-origin relative paths; everything else → "/".
// Guards against open redirects: reject anything that is not a single leading
// "/", plus "//" and "/\" (both normalize to a scheme-relative //host URL in
// browsers) and any backslash / control char (CRLF, tab) that a browser may
// normalize or that could enable header injection.
func safeReturnTo(v string) string {
	if v == "" || v[0] != '/' {
		return "/"
	}
	if strings.ContainsAny(v, "\\\r\n\t") {
		return "/"
	}
	if len(v) > 1 && v[1] == '/' {
		return "/"
	}
	return v
}

// withError appends ?error=<code> (or &error=) to a same-origin path.
func withError(path, code string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "error=" + code
}

func withProviderError(path, code, provider string) string {
	return withError(path, code) + "&provider=" + url.QueryEscape(provider)
}

// randHex returns a random hex string of n bytes (2n hex chars).
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
