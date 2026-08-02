package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ── Avatar / cover upload (account self-management) ──────────────────────────
// The IdP proxies the upload to the asset service on the caller's behalf (the
// account app is cookie-authenticated and holds no bearer of its own).

type UploadAvatarReq struct {
	g.Meta `path:"/api/v1/session/avatar" method:"post" mime:"multipart/form-data" tags:"account" summary:"Upload own avatar"`
	File   *ghttp.UploadFile `json:"file" type:"file"`
}

type UploadAvatarRes struct {
	Avatar MediaRef `json:"avatar"`
}

type UploadCoverReq struct {
	g.Meta `path:"/api/v1/session/cover" method:"post" mime:"multipart/form-data" tags:"account" summary:"Upload own cover banner"`
	File   *ghttp.UploadFile `json:"file" type:"file"`
}

type UploadCoverRes struct {
	Cover MediaRef `json:"cover"`
}
