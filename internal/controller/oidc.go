package controller

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/ory/fosite"

	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
)

// OIDCController handles the OAuth2/OIDC protocol endpoints.
type OIDCController struct {
	provider   fosite.OAuth2Provider
	keys       *oidc.Manager
	svc        *logic.Service
	issuer     string
	loginURL   string
	postLogout map[string]bool // allowed post_logout_redirect_uri origins (RP logout)
}

// SetPostLogoutRedirects registers the origins a post_logout_redirect_uri may
// point at (RP-initiated logout, end_session). Without a match the uri is
// ignored — guards against an open redirect. Prod should derive this from each
// client's registered post_logout_redirect_uris; the seeded consumer origins
// suffice for the self-operated station group.
func (c *OIDCController) SetPostLogoutRedirects(origins []string) {
	c.postLogout = map[string]bool{}
	for _, o := range origins {
		c.postLogout[strings.TrimRight(o, "/")] = true
	}
}

func (c *OIDCController) allowedPostLogout(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	return c.postLogout[u.Scheme+"://"+u.Host]
}

// NewOIDC creates an OIDCController wired to a fosite provider and key manager.
func NewOIDC(p fosite.OAuth2Provider, keys *oidc.Manager, svc *logic.Service, issuer, loginURL string) *OIDCController {
	return &OIDCController{provider: p, keys: keys, svc: svc, issuer: issuer, loginURL: loginURL}
}

// discoveryDoc builds the OIDC discovery document (testable without HTTP).
func (c *OIDCController) discoveryDoc() map[string]interface{} {
	return map[string]interface{}{
		"issuer":                                c.issuer,
		"authorization_endpoint":                c.issuer + "/oauth2/authorize",
		"token_endpoint":                        c.issuer + "/oauth2/token",
		"userinfo_endpoint":                     c.issuer + "/oauth2/userinfo",
		"jwks_uri":                              c.issuer + "/oauth2/jwks.json",
		"revocation_endpoint":                   c.issuer + "/oauth2/revoke",
		"end_session_endpoint":                  c.issuer + "/oauth2/end_session",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", "client_credentials"},
		"scopes_supported":                      []string{"openid", "profile", "email", "roles", "offline_access"},
		"code_challenge_methods_supported":      []string{"S256"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"subject_types_supported":               []string{"public"},
	}
}

// Discovery serves GET /.well-known/openid-configuration.
func (c *OIDCController) Discovery(r *ghttp.Request) {
	r.Response.WriteJson(c.discoveryDoc())
}

// JWKS serves GET /oauth2/jwks.json.
func (c *OIDCController) JWKS(r *ghttp.Request) {
	r.Response.WriteJson(c.keys.JWKS())
}

// Authorize handles GET /oauth2/authorize.
// Subject comes from the IdP login session (id_session cookie).
// No session → redirect to login.
func (c *OIDCController) Authorize(r *ghttp.Request) {
	ctx := r.Context()
	// RawWriter bypasses GoFrame's response buffer so fosite can write headers +
	// redirect directly to the wire without buffering interference.
	w := r.Response.RawWriter()
	req := r.Request

	ar, err := c.provider.NewAuthorizeRequest(ctx, req)
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}

	sid := r.Cookie.Get("id_session", "").String()
	id, merr := c.svc.Me(ctx, sid)
	if merr != nil {
		r.Response.RedirectTo(c.loginURL + "?return_to=" + url.QueryEscape(req.URL.String()))
		return
	}

	for _, scope := range ar.GetRequestedScopes() {
		ar.GrantScope(scope) // first-party: no consent UI
	}

	profile, _ := c.svc.GetProfile(ctx, id.ID) // empty profile is acceptable
	roles, _ := c.svc.GetRoles(ctx, id.ID)     // empty roles is acceptable
	session := oidc.BuildSession(
		c.issuer, ar.GetClient().GetID(), c.keys.ActiveKID(), sid,
		id, profile, ar.GetGrantedScopes(), roles, time.Now().UTC(),
	)

	resp, err := c.provider.NewAuthorizeResponse(ctx, ar, session)
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}
	c.provider.WriteAuthorizeResponse(ctx, w, ar, resp)
}

// Token handles POST /oauth2/token (authorization_code + refresh_token grants).
//
// ROLES ON REFRESH: on the refresh_token grant fosite re-hydrates the ORIGINAL
// authorize-time session from storage and deep-clones it INTO the access request
// (handler/oauth2/flow_refresh.go:87 `request.SetSession(originalRequest.
// GetSession().Clone())`), which runs inside NewAccessRequest. So when
// NewAccessRequest returns, accessReq.GetSession() is the live, hydrated
// *oidc.Session that will be signed — and strategy_jwt.go reads its JWT claims
// LIVE at signing time (during NewAccessResponse). That gives us an
// application-level seam: between NewAccessRequest and NewAccessResponse we
// re-fetch the identity's CURRENT roles and overwrite the "roles" claim, so role
// grants/revocations take effect on refresh instead of only on the
// next full authorize. Without this, a revoked admin would keep "admin" in their
// access token until the refresh token itself expired.
func (c *OIDCController) Token(r *ghttp.Request) {
	ctx := r.Context()
	w := r.Response.RawWriter()
	req := r.Request

	session := oidc.EmptySession()
	accessReq, err := c.provider.NewAccessRequest(ctx, req, session)
	if err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	c.refreshRolesClaim(ctx, accessReq)
	c.applyServiceClaims(ctx, accessReq)
	resp, err := c.provider.NewAccessResponse(ctx, accessReq)
	if err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	c.provider.WriteAccessResponse(ctx, w, accessReq, resp)
}

// refreshRolesClaim re-fetches the identity's current roles and overwrites the
// "roles" claim on the (already deep-cloned, live) refresh-grant session, so the
// freshly-signed access-token JWT — and the id token if it is re-minted — carry
// up-to-date roles. No-op unless this is a refresh_token grant that was granted
// the "roles" scope. The roles claim must mirror BuildSession's authorize-time
// shape (a non-nil slice). It is a no-op on the authorization_code grant, whose
// session already holds fresh authorize-time roles.
func (c *OIDCController) refreshRolesClaim(ctx context.Context, accessReq fosite.AccessRequester) {
	if !accessReq.GetGrantTypes().ExactOne("refresh_token") || !accessReq.GetGrantedScopes().Has("roles") {
		return
	}
	sess, ok := accessReq.GetSession().(*oidc.Session)
	if !ok || sess.JWTClaims == nil {
		return
	}
	roles, err := c.svc.GetRoles(ctx, sess.JWTClaims.Subject)
	if err != nil {
		return // keep the cloned authorize-time roles rather than dropping them
	}
	if roles == nil {
		roles = []string{}
	}
	// Access-token JWT (primary: resource servers read this). Extra is non-nil
	// here because BuildSession set accessExtra["roles"] when the roles scope was
	// granted, and the clone preserves it — but guard anyway.
	if sess.JWTClaims.Extra == nil {
		sess.JWTClaims.Extra = map[string]interface{}{}
	}
	sess.JWTClaims.Extra["roles"] = roles
	// ID token (secondary: only re-minted when openid is granted). Only touch a
	// non-nil claims map to avoid introducing a nil-map panic.
	if sess.DefaultSession != nil && sess.DefaultSession.Claims != nil && sess.DefaultSession.Claims.Extra != nil {
		sess.DefaultSession.Claims.Extra["roles"] = roles
	}
}

// applyServiceClaims stamps the service identity onto a client_credentials
// access token. fosite's client_credentials handler grants the requested scopes
// (they land in the JWT "scope" claim automatically) but never sets a subject —
// there is no user. We set sub = client_id so the token names the calling
// service, and copy the active "kid" into the JWT header so resource servers can
// resolve the signing key (EmptySession carries no kid). No-op for every other
// grant, whose session already holds an authenticated subject + kid.
func (c *OIDCController) applyServiceClaims(_ context.Context, accessReq fosite.AccessRequester) {
	if !accessReq.GetGrantTypes().ExactOne("client_credentials") {
		return
	}
	// fosite's client_credentials handler validates the requested scopes against
	// the client's allowlist but does NOT itself grant them, so the access token
	// would carry an empty "scope" claim. Grant the (already validated) requested
	// scopes here so resource servers can authorize on them (e.g. asset:sign).
	for _, sc := range accessReq.GetRequestedScopes() {
		accessReq.GrantScope(sc)
	}
	sess, ok := accessReq.GetSession().(*oidc.Session)
	if !ok {
		return
	}
	clientID := accessReq.GetClient().GetID()
	if sess.JWTClaims != nil {
		sess.JWTClaims.Subject = clientID
	}
	if sess.DefaultSession != nil {
		sess.DefaultSession.Subject = clientID
	}
	if sess.JWTHeader != nil {
		if sess.JWTHeader.Extra == nil {
			sess.JWTHeader.Extra = map[string]interface{}{}
		}
		sess.JWTHeader.Extra["kid"] = c.keys.ActiveKID()
	}
}

// Userinfo handles GET/POST /oauth2/userinfo.
func (c *OIDCController) Userinfo(r *ghttp.Request) {
	ctx := r.Context()
	req := r.Request

	session := oidc.EmptySession()
	tokenType, ar, err := c.provider.IntrospectToken(
		ctx, fosite.AccessTokenFromRequest(req), fosite.AccessToken, session,
	)
	if err != nil || tokenType != fosite.AccessToken {
		r.Response.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		r.Response.WriteHeader(401)
		r.Response.WriteJson(map[string]string{"error": "invalid_token"})
		return
	}

	sub := ar.GetSession().GetSubject()
	id, err := c.svc.GetByID(ctx, sub)
	if err != nil {
		r.Response.WriteHeader(401)
		r.Response.WriteJson(map[string]string{"error": "invalid_token"})
		return
	}
	profile, _ := c.svc.GetProfile(ctx, sub)
	roles, _ := c.svc.GetRoles(ctx, sub)
	r.Response.WriteJson(oidc.Userinfo(id, profile, ar.GetGrantedScopes(), roles))
}

// Revoke handles POST /oauth2/revoke (RFC 7009). Raw RFC response.
func (c *OIDCController) Revoke(r *ghttp.Request) {
	ctx := r.Context()
	w := r.Response.RawWriter()
	err := c.provider.NewRevocationRequest(ctx, r.Request)
	c.provider.WriteRevocationResponse(ctx, w, err)
}

// EndSession handles GET/POST /oauth2/end_session (RP-initiated logout, MVP).
// Clears the IdP login session and revokes refresh tokens bound to it (via
// svc.Logout, which is session-bound). Does NOT honor arbitrary
// post_logout_redirect_uri (open-redirect guard; whitelist redirect is future work).
func (c *OIDCController) EndSession(r *ghttp.Request) {
	ctx := r.Context()
	sid := r.Cookie.Get("id_session", "").String()
	if sid != "" {
		_ = c.svc.Logout(ctx, sid)
		r.Cookie.Remove("id_session")
	}
	// RP-initiated logout: after clearing the IdP session, bounce back to the
	// relying party (so the user lands on the consumer site, logged out) when it
	// supplies an allow-listed post_logout_redirect_uri.
	if uri := r.GetQuery("post_logout_redirect_uri").String(); uri != "" && c.allowedPostLogout(uri) {
		r.Response.RedirectTo(uri)
		return
	}
	r.Response.WriteJson(map[string]interface{}{"logged_out": true})
}
