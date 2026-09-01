package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/ory/fosite"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/oauthlogin"
	"github.com/yueli-official/identity/internal/oidc"
	"github.com/yueli-official/identity/internal/repo"
)

// OIDCController handles the OAuth2/OIDC protocol endpoints.
type OIDCController struct {
	provider           fosite.OAuth2Provider
	keys               *oidc.Manager
	svc                *logic.Service
	issuer             string
	loginURL           string
	mediaBaseURL       string
	clients            repo.ClientRepo
	secureCookie       bool
	reauthSecret       []byte
	refreshReplayStore oidc.RefreshReplayStore
	refreshReplayCodec *oidc.RefreshReplayCodec
	refreshReplayGrace time.Duration
	now                func() time.Time
}

const (
	oidcReauthCookie = "id_oidc_reauth"
	oidcReauthParam  = "_identity_reauth"
	oidcReauthTTL    = 10 * time.Minute
)

func (c *OIDCController) allowedPostLogout(ctx context.Context, clientID, uri string) bool {
	if clientID == "" || uri == "" || c.clients == nil {
		return false
	}
	client, err := c.clients.GetClient(ctx, clientID)
	return err == nil && slices.Contains(client.PostLogoutRedirectURIs, uri)
}

// NewOIDC creates an OIDCController wired to a fosite provider and key manager.
func NewOIDC(
	p fosite.OAuth2Provider,
	keys *oidc.Manager,
	svc *logic.Service,
	clients repo.ClientRepo,
	issuer, loginURL, mediaBaseURL string,
	secureCookie bool,
	reauthSecret []byte,
) *OIDCController {
	controller := &OIDCController{
		provider: p, keys: keys, svc: svc, clients: clients,
		issuer: issuer, loginURL: loginURL, mediaBaseURL: mediaBaseURL, secureCookie: secureCookie,
		reauthSecret: slices.Clone(reauthSecret),
		now:          time.Now,
	}
	return controller
}

func (c *OIDCController) EnableRefreshReplay(store oidc.RefreshReplayStore, secret []byte, grace time.Duration) error {
	if store == nil || grace <= 0 {
		return errors.New("refresh replay store and positive grace are required")
	}
	codec, err := oidc.NewRefreshReplayCodec(secret)
	if err != nil {
		return err
	}
	c.refreshReplayStore, c.refreshReplayCodec, c.refreshReplayGrace = store, codec, grace
	return nil
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
		"subject_types_supported":               []string{"public", "pairwise"},
		"acr_values_supported": []string{
			string(authentication.ProfileBaseline),
			string(authentication.ProfileMultiFactor),
			string(authentication.ProfilePhishingResistant),
		},
		"claims_supported": []string{
			"sub", "user_key", "iss", "aud", "exp", "iat", "auth_time", "acr", "amr",
			"name", "preferred_username", "picture", "locale", "email", "email_verified", "roles",
		},
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
	idpSession, id, merr := c.svc.AuthenticatedSession(ctx, sid)
	if merr != nil {
		if promptContains(ar.GetRequestForm(), "none") {
			c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrLoginRequired)
			return
		}
		c.redirectToLogin(r)
		return
	}

	reauthenticated := c.consumeReauthMarker(r, idpSession)
	needed, requirementErr := activeAuthenticationRequired(
		ar.GetRequestForm(), idpSession, time.Now().UTC(),
	)
	if requirementErr != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, requirementErr)
		return
	}
	if needed && !reauthenticated {
		if promptContains(ar.GetRequestForm(), "none") {
			c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrLoginRequired)
			return
		}
		if err := c.redirectToReauthentication(r); err != nil {
			c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrServerError)
		}
		return
	}
	if !authenticationContextAccepted(ar.GetRequestForm(), idpSession) {
		if reauthenticated || promptContains(ar.GetRequestForm(), "none") {
			c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrLoginRequired)
			return
		}
		if err := c.redirectToReauthentication(r); err != nil {
			c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrServerError)
		}
		return
	}

	for _, scope := range ar.GetRequestedScopes() {
		ar.GrantScope(scope) // first-party: no consent UI
	}
	audiences := c.clientAudiences(ctx, ar.GetClient().GetID())
	for _, audience := range audiences {
		ar.GrantAudience(audience)
	}

	profile, _ := c.svc.GetProfile(ctx, id.ID) // empty profile is acceptable
	roles, _ := c.svc.GetRoles(ctx, id.ID)     // empty roles is acceptable
	client, err := c.clients.GetClient(ctx, ar.GetClient().GetID())
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrServerError)
		return
	}
	subject, err := c.svc.OIDCSubject(ctx, id.ID, client)
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrServerError)
		return
	}
	session := oidc.BuildSession(
		c.issuer, c.mediaBaseURL, ar.GetClient().GetID(), c.keys.ActiveKID(), sid,
		subject, id, profile, ar.GetGrantedScopes(), roles, idpSession.Authentication, time.Now().UTC(),
	)
	session.JWTClaims.Audience = audiences

	resp, err := c.provider.NewAuthorizeResponse(ctx, ar, session)
	if err != nil {
		c.provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}
	c.provider.WriteAuthorizeResponse(ctx, w, ar, resp)
}

func activeAuthenticationRequired(
	form url.Values,
	session model.Session,
	now time.Time,
) (bool, error) {
	if promptContains(form, "login") {
		return true, nil
	}
	rawMaxAge := strings.TrimSpace(form.Get("max_age"))
	if rawMaxAge == "" {
		return false, nil
	}
	maxAge, err := strconv.ParseInt(rawMaxAge, 10, 64)
	if err != nil || maxAge < 0 {
		return false, fosite.ErrInvalidRequest.WithHint("max_age must be a non-negative integer")
	}
	if maxAge == 0 || session.Authentication.AuthenticatedAt.IsZero() ||
		session.Authentication.AuthenticatedAt.After(now) {
		return true, nil
	}
	return now.Sub(session.Authentication.AuthenticatedAt) > time.Duration(maxAge)*time.Second, nil
}

func promptContains(form url.Values, expected string) bool {
	return slices.Contains(strings.Fields(form.Get("prompt")), expected)
}

func authenticationContextAccepted(form url.Values, session model.Session) bool {
	requested := strings.Fields(form.Get("acr_values"))
	return len(requested) == 0 || slices.Contains(requested, string(session.Authentication.Profile))
}

func (c *OIDCController) redirectToLogin(r *ghttp.Request) {
	r.Response.RedirectTo(c.loginURL + "?return_to=" + url.QueryEscape(r.Request.URL.RequestURI()))
}

func (c *OIDCController) redirectToReauthentication(r *ghttp.Request) error {
	if len(c.reauthSecret) < 32 {
		return errors.New("OIDC reauthentication secret is not configured")
	}
	now := time.Now().UTC()
	original, err := removeQueryValue(r.Request.URL.RequestURI(), oidcReauthParam)
	if err != nil {
		return err
	}
	nonce := randHex(16)
	marker := oauthlogin.EncodeState(
		c.reauthSecret, original, nonce, strconv.FormatInt(now.Unix(), 10),
		now.Add(oidcReauthTTL).Unix(),
	)
	r.Cookie.SetHttpCookie(&http.Cookie{
		Name: oidcReauthCookie, Value: marker, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode,
		MaxAge: int(oidcReauthTTL.Seconds()),
	})
	returnTo, err := addQueryValue(original, oidcReauthParam, nonce)
	if err != nil {
		return err
	}
	r.Response.RedirectTo(c.loginURL + "?return_to=" + url.QueryEscape(returnTo))
	return nil
}

func (c *OIDCController) consumeReauthMarker(r *ghttp.Request, session model.Session) bool {
	nonce := r.Request.URL.Query().Get(oidcReauthParam)
	if nonce == "" {
		return false
	}
	marker := r.Cookie.Get(oidcReauthCookie, "").String()
	r.Cookie.Remove(oidcReauthCookie)
	original, expectedNonce, issuedAtRaw, err := oauthlogin.DecodeState(c.reauthSecret, marker)
	if err != nil || nonce != expectedNonce {
		return false
	}
	current, err := removeQueryValue(r.Request.URL.RequestURI(), oidcReauthParam)
	if err != nil || current != original {
		return false
	}
	issuedAtUnix, err := strconv.ParseInt(issuedAtRaw, 10, 64)
	if err != nil {
		return false
	}
	// One second of database/browser timestamp precision tolerance; an old
	// session cannot satisfy a marker minted after its authentication event.
	return !session.Authentication.AuthenticatedAt.Before(time.Unix(issuedAtUnix-1, 0).UTC())
}

func addQueryValue(raw, key, value string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI(), nil
}

func removeQueryValue(raw, key string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Del(key)
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI(), nil
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
	refresh, clientID := refreshReplayInput(req)
	if refresh != "" && c.refreshReplayStore != nil {
		key := c.refreshReplayCodec.Digest(clientID, refresh)
		receipt, found, replayErr := c.refreshReplayStore.GetRefreshReplay(ctx, key, clientID, c.now().UTC())
		if replayErr != nil {
			writeTokenJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"temporarily_unavailable"}`))
			return
		}
		if found {
			body, err := c.refreshReplayCodec.Open(key, receipt.ResponseCiphertext)
			if err != nil {
				writeTokenJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"temporarily_unavailable"}`))
				return
			}
			writeTokenJSON(w, http.StatusOK, body)
			return
		}
	}

	session := oidc.EmptySession()
	accessReq, err := c.provider.NewAccessRequest(ctx, req, session)
	if err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	if err := c.refreshRolesClaim(ctx, accessReq); err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	c.applyServiceClaims(ctx, accessReq)
	resp, err := c.provider.NewAccessResponse(ctx, accessReq)
	if err != nil {
		c.provider.WriteAccessError(ctx, w, accessReq, err)
		return
	}
	if refresh != "" && c.refreshReplayStore != nil {
		body, marshalErr := json.Marshal(resp.ToMap())
		if marshalErr != nil {
			c.provider.WriteAccessError(ctx, w, accessReq, fosite.ErrServerError.WithWrap(marshalErr))
			return
		}
		key := c.refreshReplayCodec.Digest(clientID, refresh)
		ciphertext, sealErr := c.refreshReplayCodec.Seal(key, body)
		if sealErr == nil {
			receiptContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_ = c.refreshReplayStore.PutRefreshReplay(receiptContext, oidc.RefreshReplayReceipt{KeyDigest: key, ClientID: clientID, RequestID: accessReq.GetID(), ResponseCiphertext: ciphertext, ExpiresAt: c.now().UTC().Add(c.refreshReplayGrace)})
			cancel()
		}
		writeTokenJSON(w, http.StatusOK, body)
		return
	}
	c.provider.WriteAccessResponse(ctx, w, accessReq, resp)
}

func refreshReplayInput(request *http.Request) (string, string) {
	if request == nil || request.ParseForm() != nil || request.Form.Get("grant_type") != "refresh_token" {
		return "", ""
	}
	clientID := request.Form.Get("client_id")
	if basicID, _, ok := request.BasicAuth(); clientID == "" && ok {
		clientID = basicID
	}
	return request.Form.Get("refresh_token"), clientID
}
func writeTokenJSON(writer http.ResponseWriter, status int, body []byte) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writer.Header().Set("Content-Type", "application/json;charset=UTF-8")
	writer.WriteHeader(status)
	_, _ = writer.Write(body)
}

// refreshRolesClaim re-fetches the identity's current roles and overwrites the
// "roles" claim on the (already deep-cloned, live) refresh-grant session, so the
// freshly-signed access-token JWT — and the id token if it is re-minted — carry
// up-to-date roles. Every refresh grant revalidates the current account
// lifecycle, even when the client did not request roles. When roles were
// granted, the claim mirrors BuildSession's non-nil authorize-time shape.
func (c *OIDCController) refreshRolesClaim(ctx context.Context, accessReq fosite.AccessRequester) error {
	if !accessReq.GetGrantTypes().ExactOne("refresh_token") {
		return nil
	}
	sess, ok := accessReq.GetSession().(*oidc.Session)
	if !ok || sess.JWTClaims == nil {
		return fosite.ErrInvalidGrant
	}
	identity, err := c.svc.GetByOIDCSubject(ctx, sess.JWTClaims.Subject)
	if errors.Is(err, repo.ErrIdentityMissing) {
		return fosite.ErrInvalidGrant
	}
	if err != nil {
		return fosite.ErrTemporarilyUnavailable.WithWrap(err)
	}
	if identity.Status != model.StatusActive {
		return fosite.ErrInvalidGrant
	}
	if !accessReq.GetGrantedScopes().Has("roles") {
		return nil
	}
	roles, err := c.svc.GetRoles(ctx, identity.ID)
	if err != nil {
		return fosite.ErrTemporarilyUnavailable.WithWrap(err)
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
	return nil
}

// applyServiceClaims stamps the service identity onto a client_credentials
// access token. fosite's client_credentials handler grants the requested scopes
// (they land in the JWT "scope" claim automatically) but never sets a subject —
// there is no user. We set sub = client_id so the token names the calling
// service, and copy the active "kid" into the JWT header so resource servers can
// resolve the signing key (EmptySession carries no kid). No-op for every other
// grant, whose session already holds an authenticated subject + kid.
func (c *OIDCController) applyServiceClaims(ctx context.Context, accessReq fosite.AccessRequester) {
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
	for _, audience := range c.clientAudiences(ctx, clientID) {
		accessReq.GrantAudience(audience)
	}
	if sess.JWTClaims != nil {
		sess.JWTClaims.Subject = clientID
		sess.JWTClaims.Audience = c.clientAudiences(ctx, clientID)
		if sess.JWTClaims.Extra == nil {
			sess.JWTClaims.Extra = map[string]interface{}{}
		}
		sess.JWTClaims.Extra["client_id"] = clientID
		sess.JWTClaims.Extra["subject_kind"] = "client"
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

func (c *OIDCController) clientAudiences(ctx context.Context, clientID string) []string {
	if c.clients != nil {
		client, err := c.clients.GetClient(ctx, clientID)
		if err == nil && len(client.Audiences) > 0 {
			return append([]string(nil), client.Audiences...)
		}
	}
	return []string{clientID}
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
	id, err := c.svc.GetByOIDCSubject(ctx, sub)
	if err != nil || id.Status != model.StatusActive {
		r.Response.WriteHeader(401)
		r.Response.WriteJson(map[string]string{"error": "invalid_token"})
		return
	}
	profile, _ := c.svc.GetProfile(ctx, id.ID)
	roles, _ := c.svc.GetRoles(ctx, id.ID)
	r.Response.WriteJson(oidc.Userinfo(c.mediaBaseURL, sub, id, profile, ar.GetGrantedScopes(), roles))
}

// Revoke handles POST /oauth2/revoke (RFC 7009). Raw RFC response.
func (c *OIDCController) Revoke(r *ghttp.Request) {
	ctx := r.Context()
	w := r.Response.RawWriter()
	err := c.provider.NewRevocationRequest(ctx, r.Request)
	c.provider.WriteRevocationResponse(ctx, w, err)
}

// EndSession handles GET/POST /oauth2/end_session (RP-initiated logout, MVP).
// Clears the IdP login session and revokes refresh tokens bound to it. Redirects
// only when client_id owns the exact registered post_logout_redirect_uri.
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
	uri := r.GetQuery("post_logout_redirect_uri").String()
	clientID := r.GetQuery("client_id").String()
	if c.allowedPostLogout(ctx, clientID, uri) {
		r.Response.RedirectTo(uri)
		return
	}
	r.Response.WriteJson(map[string]interface{}{"logged_out": true})
}
