package controller

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"platform/gokit/response"
	"platform/services/identity/internal/oidc"
)

// AssetAdminProxy forwards account-console asset admin requests to the asset
// service after verifying the cookie-authenticated caller is a global admin.
type AssetAdminProxy struct {
	base   *Controller
	mgr    *oidc.Manager
	issuer string
	asset  string
}

func NewAssetAdminProxy(base *Controller, mgr *oidc.Manager, issuer, assetBaseURL string) *AssetAdminProxy {
	return &AssetAdminProxy{
		base: base, mgr: mgr, issuer: issuer, asset: strings.TrimRight(assetBaseURL, "/"),
	}
}

func (p *AssetAdminProxy) Forward(r *ghttp.Request) {
	adminID, err := p.base.requireAdmin(r.Context())
	if err != nil {
		r.SetError(err)
		return
	}
	bearer, err := p.mgr.MintServiceToken(p.issuer, adminID, "asset:sign", 2*time.Minute, time.Now())
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
	var raw string
	switch r.Method {
	case "GET":
		resp, e := cli.Get(r.Context(), target)
		err = e
		if err == nil {
			defer resp.Close()
			raw = resp.ReadAllString()
		}
	case "POST":
		var body []byte
		body, err = io.ReadAll(r.Request.Body)
		if err == nil {
			resp, e := cli.Post(r.Context(), target, body)
			err = e
			if err == nil {
				defer resp.Close()
				raw = resp.ReadAllString()
			}
		}
	case "DELETE":
		resp, e := cli.Delete(r.Context(), target)
		err = e
		if err == nil {
			defer resp.Close()
			raw = resp.ReadAllString()
		}
	default:
		r.Response.WriteStatus(405)
		return
	}
	if err != nil {
		r.SetError(fmt.Errorf("asset service unreachable: %w", err))
		return
	}
	j := gjson.New(raw)
	if code := j.Get("code").String(); code != "ok" {
		r.Response.WriteJson(response.Fail(code, j.Get("message").String(), nil))
		return
	}
	r.Response.WriteJson(response.OK(j.Get("data").Val()))
}
