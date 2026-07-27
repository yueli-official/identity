package controller

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/gclient"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/yueli-official/identity/internal/oidc"
)

// AssetAdminProxy forwards account-console asset admin requests to the asset
// service after verifying the cookie-authenticated caller is a global admin.
type AssetAdminProxy struct {
	base     *Controller
	mgr      *oidc.Manager
	issuer   string
	asset    string
	audience string
}

func NewAssetAdminProxy(base *Controller, mgr *oidc.Manager, issuer, assetBaseURL, audience string) *AssetAdminProxy {
	return &AssetAdminProxy{
		base: base, mgr: mgr, issuer: issuer, asset: strings.TrimRight(assetBaseURL, "/"), audience: audience,
	}
}

func (p *AssetAdminProxy) Forward(r *ghttp.Request) {
	adminID, err := p.base.requireAdmin(r.Context())
	if err != nil {
		r.SetError(err)
		return
	}
	bearer, err := p.mgr.MintServiceToken(p.issuer, adminID, p.audience, "asset:sign", 2*time.Minute, time.Now())
	if err != nil {
		r.SetError(err)
		return
	}
	suffix := strings.TrimPrefix(r.Request.URL.Path, "/api/v1/admin/assets-proxy")
	if suffix == "" {
		suffix = "/stats"
	}
	target := p.asset + "/api/v1/admin/assets" + suffix
	if raw := r.Request.URL.RawQuery; raw != "" {
		target += "?" + raw
	}

	cli := g.Client()
	cli.SetHeader("Authorization", "Bearer "+bearer)
	cli.ContentJson()
	var upstream *gclient.Response
	switch r.Method {
	case "GET":
		upstream, err = cli.Get(r.Context(), target)
	case "POST":
		var body []byte
		body, err = io.ReadAll(r.Request.Body)
		if err == nil {
			upstream, err = cli.Post(r.Context(), target, body)
		}
	case "DELETE":
		upstream, err = cli.Delete(r.Context(), target)
	default:
		r.Response.WriteStatus(405)
		return
	}
	if err != nil {
		r.SetError(fmt.Errorf("asset service unreachable: %w", err))
		return
	}
	defer upstream.Close()
	body := upstream.ReadAll()
	contentType := upstream.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") && !strings.HasPrefix(contentType, "application/problem+json") {
		r.SetError(fmt.Errorf("asset service returned unsupported content type"))
		return
	}
	r.Response.Header().Set("Content-Type", contentType)
	if traceID := upstream.Header.Get("X-Trace-Id"); traceID != "" {
		r.Response.Header().Set("X-Trace-Id", traceID)
	}
	r.Response.WriteHeader(upstream.StatusCode)
	r.Response.Write(body)
}
