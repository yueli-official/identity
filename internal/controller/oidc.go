package controller

import (
	"net/url"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/ory/fosite"

	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
)

// OIDCController handles the OAuth2/OIDC protocol endpoints.
type OIDCController struct {
	provider fosite.OAuth2Provider
	keys     *oidc.Manager
	svc      *logic.Service
	issuer   string
	loginURL string
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
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
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
// Subject comes from the IdP login session (id_session cookie from milestone ②).
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
	session := oidc.BuildSession(
		c.issuer, ar.GetClient().GetID(), c.keys.ActiveKID(), sid,
		id, profile, ar.GetGrantedScopes(), time.Now().UTC(),
	)

	resp, err := c.provider.NewAuthorizeResponse(ctx, ar, session)
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}
	c.provider.WriteAuthorizeResponse(ctx, w, ar, resp)
}

// Token handles POST /oauth2/token.
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
	resp, err := c.provider.NewAccessResponse(ctx, accessReq)
	if err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	c.provider.WriteAccessResponse(ctx, w, accessReq, resp)
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
	r.Response.WriteJson(oidc.Userinfo(id, profile, ar.GetGrantedScopes()))
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
// svc.Logout, which milestone ④ makes session-bound). Does NOT honor arbitrary
// post_logout_redirect_uri (open-redirect guard; whitelist redirect is ⑦).
func (c *OIDCController) EndSession(r *ghttp.Request) {
	ctx := r.Context()
	sid := r.Cookie.Get("id_session", "").String()
	if sid != "" {
		_ = c.svc.Logout(ctx, sid)
		r.Cookie.Remove("id_session")
	}
	r.Response.WriteJson(map[string]interface{}{"logged_out": true})
}
