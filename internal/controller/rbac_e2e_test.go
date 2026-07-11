package controller_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
)

// TestE2E_RBAC is the integration witness that ties the admin role
// endpoint and the OIDC token issuance together over real HTTP, hermetically.
//
// It deliberately fills the gap left by the two existing tests:
//
//   - internal/oidc/flow_test.go::TestOIDCRefreshRolesUpdated already proves the
//     default [user] roles claim on a fresh authorize, and that a grant (made via
//     svc.GrantRole DIRECTLY) is reflected on the *REFRESH* token path.
//   - admin_e2e_test.go::TestAdminRoleEndpoint already proves the admin endpoint
//     authz matrix (401/403/400/200, no mutation on reject).
//
// Neither exercises the chain proved here: granting admin through the REAL admin
// HTTP endpoint using a REAL admin session (controller authz path end-to-end),
// then re-running a BRAND-NEW authorize+token flow and asserting the freshly
// minted access token now carries "admin". This proves the fresh-authorize issue
// path re-reads roles at issue time (distinct from the refresh path), and that
// the grant performed via the controller — not the service seam — propagates all
// the way into a signed JWT.
//
// The whole stack (admin/business controller + OIDC controller) runs on one
// in-memory-backed GoFrame server; no live DB or network.
func TestE2E_RBAC(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		const clientID = "demo-web"
		const scope = "openid profile email roles"
		ctx := context.Background()

		// --- in-memory repo + demo OIDC client (authorization_code only) --------
		r := repo.NewMemory()
		r.SetClient(model.OIDCClient{
			ID:            clientID,
			Public:        true,
			RedirectURIs:  []string{rbacCallbackURI},
			GrantTypes:    []string{"authorization_code"},
			ResponseTypes: []string{"code"},
			Scopes:        []string{"openid", "profile", "email", "roles"},
		})

		svc := logic.New(r, logic.DefaultConfig())

		// --- seed an admin (granted via the service seam, mirroring main.go's
		//     bootstrap) and a target user; both registered + logged-in. ---------
		mkUser := func(email string) (id, sid string) {
			out, err := svc.Register(ctx, logic.RegisterInput{
				Email: email, Password: "longenough123", DisplayName: email,
			})
			t.AssertNil(err)
			lo, err := svc.Login(ctx, logic.LoginInput{Email: email, Password: "longenough123", IP: "127.0.0.1"})
			t.AssertNil(err)
			return out.ID, lo.SessionID
		}

		adminID, adminSID := mkUser("admin@e.com")
		t.AssertNil(svc.GrantRole(ctx, adminID, logic.AdminRole))
		targetID, targetSID := mkUser("target@e.com")

		// --- pick a free port so the issuer URL exists before the server starts.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		t.AssertNil(err)
		port := ln.Addr().(*net.TCPAddr).Port
		t.AssertNil(ln.Close())
		base := fmt.Sprintf("http://127.0.0.1:%d", port)

		// --- OIDC stack (manager -> store -> provider -> controller) ------------
		mgr, err := oidc.NewManager(ctx, r)
		t.AssertNil(err)
		store := oidc.NewStore(oidc.NewMemBackend(), r)
		provider := oidc.NewProvider(store, oidc.Config{
			Issuer:       base,
			GlobalSecret: []byte("0123456789abcdef0123456789abcdef"),
			AccessTTL:    10 * time.Minute,
			IDTTL:        10 * time.Minute,
		}, mgr.KeyGetter)
		oidcCtl := controller.NewOIDC(provider, mgr, svc, r, base, base+"/login")

		// business controller carries the admin endpoints; secureCookie=false so
		// the session cookie round-trips over plain HTTP.
		bizCtl := controller.New(svc, false)

		// --- one server: business API (with envelope middleware) + raw OIDC -----
		s := g.Server(t.Name())
		s.SetAddr(fmt.Sprintf("127.0.0.1:%d", port)) // loopback-only: avoids the Windows Firewall prompt
		s.SetDumpRouterMap(false)
		s.Group("/", func(grp *ghttp.RouterGroup) {
			grp.Middleware(ghttpx.Middleware)
			grp.Bind(bizCtl)
		})
		// OIDC handlers: NO envelope middleware (fosite writes raw RFC responses).
		s.BindHandler("GET:/oauth2/jwks.json", oidcCtl.JWKS)
		s.BindHandler("GET:/oauth2/authorize", oidcCtl.Authorize)
		s.BindHandler("POST:/oauth2/token", oidcCtl.Token)
		s.Start()
		defer s.Shutdown()

		// =====================================================================
		// 1. Fresh authorize+token for the target → default roles == [user].
		//    (controller-package witness that a fresh issue reads roles, with the
		//     default applied; flow_test.go proves the same in the oidc package.)
		// =====================================================================
		roles0 := authorizeTokenRoles(t, ctx, base, clientID, scope, targetSID)
		t.Assert(rbacRolesContain(roles0, "user"), true)
		t.Assert(rbacRolesContain(roles0, "admin"), false)

		// =====================================================================
		// 2. Grant "admin" to the target via the REAL admin HTTP endpoint using
		//    the admin session (exercises requireAdmin -> GrantRole end-to-end).
		// =====================================================================
		grantPath := base + "/api/v1/admin/identities/" + targetID + "/roles?role=admin"
		grantReq, err := http.NewRequestWithContext(ctx, http.MethodPost, grantPath, nil)
		t.AssertNil(err)
		grantReq.Header.Set("Cookie", "id_session="+adminSID)
		grantResp, err := http.DefaultClient.Do(grantReq)
		t.AssertNil(err)
		grantBody, _ := io.ReadAll(grantResp.Body)
		grantResp.Body.Close()
		t.Assert(grantResp.StatusCode, 200)
		gj := gjson.New(grantBody)
		t.Assert(gj.Get("code").String(), "ok")
		// The endpoint response itself must list the updated roles incl. admin.
		respRoles := gj.Get("data.roles").Strings()
		t.Assert(rbacRolesContain(respRoles, "admin"), true)
		t.Assert(rbacRolesContain(respRoles, "user"), true)

		// =====================================================================
		// 3. BRAND-NEW authorize+token for the target → roles now contain admin.
		//    This is the genuine new property: a fresh authorize re-reads roles at
		//    issue time, so the controller-side grant lands in a freshly signed
		//    JWT (NOT the refresh path covered elsewhere).
		// =====================================================================
		roles1 := authorizeTokenRoles(t, ctx, base, clientID, scope, targetSID)
		t.Assert(rbacRolesContain(roles1, "admin"), true)
		t.Assert(rbacRolesContain(roles1, "user"), true)
	})
}

// rbacCallbackURI is the redirect_uri for the demo client seeded above.
const rbacCallbackURI = "http://127.0.0.1/callback"

// authorizeTokenRoles drives a full PKCE authorize+token exchange for the session
// behind sid, then RS256-verifies the resulting access token against the IdP JWKS
// and returns its "roles" claim. Hermetic: JWKS is fetched from the same server.
func authorizeTokenRoles(t *gtest.T, ctx context.Context, base, clientID, scope, sid string) []string {
	// PKCE verifier + S256 challenge.
	vb := make([]byte, 48)
	_, err := rand.Read(vb)
	t.AssertNil(err)
	verifier := base64.RawURLEncoding.EncodeToString(vb)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// --- authorize: read the code out of the 302 Location ---
	authURL := base + "/oauth2/authorize?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {rbacCallbackURI},
		"scope":                 {scope},
		"state":                 {"xyz-rbac-state"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()
	authReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	t.AssertNil(err)
	authReq.Header.Set("Cookie", "id_session="+sid)
	authResp, err := noRedirect.Do(authReq)
	t.AssertNil(err)
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusFound && authResp.StatusCode != http.StatusSeeOther {
		ab, _ := io.ReadAll(authResp.Body)
		t.Fatalf("authorize: expected 302/303, got %d: %s", authResp.StatusCode, ab)
	}
	loc, err := url.Parse(authResp.Header.Get("Location"))
	t.AssertNil(err)
	if !strings.HasPrefix(loc.String(), rbacCallbackURI) {
		t.Fatalf("authorize: location does not start with callback URI, got %q", loc.String())
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("authorize: no code in redirect: %s", loc.String())
	}

	// --- token: exchange the code ---
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {rbacCallbackURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	tokReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/oauth2/token", strings.NewReader(form.Encode()))
	t.AssertNil(err)
	tokReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokResp, err := http.DefaultClient.Do(tokReq)
	t.AssertNil(err)
	defer tokResp.Body.Close()
	tokBody, _ := io.ReadAll(tokResp.Body)
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token: expected 200, got %d: %s", tokResp.StatusCode, tokBody)
	}
	var ts struct {
		AccessToken string `json:"access_token"`
	}
	t.AssertNil(json.Unmarshal(tokBody, &ts))
	if ts.AccessToken == "" {
		t.Fatalf("token: empty access_token in: %s", tokBody)
	}

	return rbacDecodeAccessTokenRoles(t, base, ts.AccessToken)
}

// rbacDecodeAccessTokenRoles fetches the IdP JWKS, RS256-verifies the access token
// offline (resource-server style), and returns its "roles" claim as a []string.
// Mirrors the decode helper in internal/oidc/flow_test.go (not importable from the
// controller_test package).
func rbacDecodeAccessTokenRoles(t *gtest.T, idpURL, accessToken string) []string {
	jwksResp, err := http.Get(idpURL + "/oauth2/jwks.json") //nolint:noctx
	t.AssertNil(err)
	defer jwksResp.Body.Close()
	var jwks jose.JSONWebKeySet
	t.AssertNil(json.NewDecoder(jwksResp.Body).Decode(&jwks))

	parsed, err := josejwt.ParseSigned(accessToken)
	t.AssertNil(err)
	if len(parsed.Headers) == 0 {
		t.Fatalf("access token: no JOSE headers")
	}
	matched := jwks.Key(parsed.Headers[0].KeyID)
	if len(matched) == 0 {
		t.Fatalf("no JWKS key matches token kid %q", parsed.Headers[0].KeyID)
	}
	var claims map[string]interface{}
	t.AssertNil(parsed.Claims(matched[0].Key, &claims))

	raw, ok := claims["roles"]
	if !ok {
		t.Fatalf("access token has no 'roles' claim: %v", claims)
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("access token 'roles' claim is not an array: %T (%v)", raw, raw)
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("access token 'roles' element not a string: %T (%v)", v, v)
		}
		out = append(out, s)
	}
	return out
}

// rbacRolesContain reports whether roles includes target.
func rbacRolesContain(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}
