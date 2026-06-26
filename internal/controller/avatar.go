package controller

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/assetclient"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
)

// maxImageBytes caps a single avatar/cover upload (the account UI already crops
// to a small JPEG, so this is a guard rail, not a tuning knob).
const maxImageBytes = 8 << 20 // 8 MiB

// serviceTokenTTL is how long the IdP-minted upload bearer lives — just long
// enough for the init → PUT → finalize round-trip.
const serviceTokenTTL = 2 * time.Minute

// AvatarController proxies avatar/cover uploads to the asset service on behalf of
// the cookie-authenticated caller. The IdP mints a short-lived user token and
// drives the upload server-side, then commits the public URL to the profile.
type AvatarController struct {
	svc    *logic.Service
	mgr    *oidc.Manager
	asset  *assetclient.Client
	issuer string
}

func NewAvatar(svc *logic.Service, mgr *oidc.Manager, asset *assetclient.Client, issuer string) *AvatarController {
	return &AvatarController{svc: svc, mgr: mgr, asset: asset, issuer: issuer}
}

// UploadAvatar stores the caller's avatar (square) and returns its public URL.
func (c *AvatarController) UploadAvatar(ctx context.Context, req *v1.UploadAvatarReq) (*v1.UploadAvatarRes, error) {
	url, err := c.upload(ctx, "avatar", req.File)
	if err != nil {
		return nil, err
	}
	return &v1.UploadAvatarRes{AvatarURL: url}, nil
}

// UploadCover stores the caller's cover banner and returns its public URL.
func (c *AvatarController) UploadCover(ctx context.Context, req *v1.UploadCoverReq) (*v1.UploadCoverRes, error) {
	url, err := c.upload(ctx, "cover", req.File)
	if err != nil {
		return nil, err
	}
	return &v1.UploadCoverRes{CoverURL: url}, nil
}

func (c *AvatarController) upload(ctx context.Context, kind string, file *ghttp.UploadFile) (string, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return "", err
	}
	if file == nil {
		return "", iderr.InvalidProfile("no file uploaded")
	}
	if file.Size > maxImageBytes {
		return "", iderr.InvalidProfile("image too large")
	}
	mime := file.Header.Get("Content-Type")
	if !strings.HasPrefix(mime, "image/") {
		return "", iderr.InvalidProfile("not an image")
	}
	f, err := file.Open()
	if err != nil {
		return "", iderr.InvalidProfile("cannot read upload")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", iderr.InvalidProfile("cannot read upload")
	}

	bearer, err := c.mgr.MintServiceToken(c.issuer, id.ID, "", serviceTokenTTL, time.Now())
	if err != nil {
		return "", err
	}
	view, err := c.asset.Upload(ctx, bearer, assetclient.InitInput{
		Filename: file.Filename, Mime: mime, Size: file.Size,
		Category: kind, Visibility: "public",
	}, data)
	if err != nil {
		return "", err
	}
	if err := c.svc.SetProfileImage(ctx, id.ID, kind, view.CdnURL); err != nil {
		return "", err
	}
	return view.CdnURL, nil
}
