package controller

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oauthlogin"
)

// oauthStateCookie holds the short-lived CSRF nonce that must match the nonce
// signed into the OAuth state parameter at /callback.
const oauthStateCookie = "g_oauth_state"

// OAuthController handles the external-provider (Google) login redirect dance.
// These are raw redirect endpoints that bypass the gokit JSON envelope, the same
// precedent as the OIDC endpoints.
type OAuthController struct {
	svc          *logic.Service
	google       oauthlogin.Provider // nil when unconfigured
	secureCookie bool
	sessionTTL   time.Duration
	stateSecret  []byte
	loginURL     string
}

// NewOAuth builds the OAuth controller. google may be nil when no credentials are
// configured, in which case the endpoints redirect to the login page with an error.
func NewOAuth(svc *logic.Service, google oauthlogin.Provider, secureCookie bool, stateSecret []byte, loginURL string, sessionTTL ...time.Duration) *OAuthController {
	ttl := logic.DefaultConfig().SessionIdleTTL
	if len(sessionTTL) > 0 {
		ttl = sessionTTL[0]
	}
	return &OAuthController{svc: svc, google: google, secureCookie: secureCookie, sessionTTL: ttl, stateSecret: stateSecret, loginURL: loginURL}
}

// GoogleStart handles GET /api/v1/auth/oauth/google/start.
// It signs the (validated) return_to + a random nonce into the OAuth state,
// drops the nonce in a short-lived HttpOnly cookie, then redirects to Google.
func (c *OAuthController) GoogleStart(r *ghttp.Request) {
	if c.google == nil {
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
	state := oauthlogin.EncodeState(c.stateSecret, returnTo, nonce, bind, time.Now().Add(10*time.Minute).Unix())
	r.Cookie.SetHttpCookie(&http.Cookie{
		Name: oauthStateCookie, Value: nonce, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	r.Response.RedirectTo(c.google.AuthorizeURL(state))
}

// GoogleCallback handles GET /api/v1/auth/oauth/google/callback.
// It verifies the signed state + nonce cookie (CSRF), exchanges the code, fetches
// the profile, performs account-linking via OAuthLogin, mints an id_session
// cookie, and redirects back to the validated return_to.
func (c *OAuthController) GoogleCallback(r *ghttp.Request) {
	ctx := r.Context()
	if c.google == nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_unavailable")
		return
	}
	returnTo, nonce, bind, err := oauthlogin.DecodeState(c.stateSecret, r.Get("state").String())
	cookieNonce := r.Cookie.Get(oauthStateCookie, "").String()
	r.Cookie.Remove(oauthStateCookie)
	if err != nil || nonce == "" || nonce != cookieNonce {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_state")
		return
	}
	code := r.Get("code").String()
	if code == "" {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_denied")
		return
	}
	at, err := c.google.ExchangeCode(ctx, code)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_exchange")
		return
	}
	ui, err := c.google.FetchUserInfo(ctx, at)
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_userinfo")
		return
	}

	// Bind flow: link the provider to the already-logged-in identity (no new
	// session) and return to the account page. returnTo is the same-origin page
	// the account UI started the bind from.
	if bind != "" {
		if berr := c.svc.BindOAuth(ctx, bind, c.google.Name(), ui.ProviderUID, ui.Email, ui.EmailVerified); berr != nil {
			r.Response.RedirectTo(withError(returnTo, "oauth_bind"))
			return
		}
		r.Response.RedirectTo(returnTo)
		return
	}

	out, err := c.svc.OAuthLogin(ctx, logic.OAuthLoginInput{
		Provider: c.google.Name(), ProviderUID: ui.ProviderUID, Email: ui.Email,
		EmailVerified: ui.EmailVerified, DisplayName: ui.DisplayName,
		UserAgent: r.UserAgent(), IP: r.GetClientIp(),
	})
	if err != nil {
		r.Response.RedirectTo(c.loginURL + "?error=oauth_login")
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

// randHex returns a random hex string of n bytes (2n hex chars).
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
