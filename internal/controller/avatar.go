package controller

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/assetclient"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/oidc"
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
	svc      *logic.Service
	mgr      *oidc.Manager
	asset    *assetclient.Client
	issuer   string
	audience string
}

func NewAvatar(svc *logic.Service, mgr *oidc.Manager, asset *assetclient.Client, issuer, audience string) *AvatarController {
	return &AvatarController{svc: svc, mgr: mgr, asset: asset, issuer: issuer, audience: audience}
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
		return "", iderr.InvalidProfile(iderr.ProfileReasonFileRequired)
	}
	if file.Size > maxImageBytes {
		return "", iderr.InvalidProfile(iderr.ProfileReasonImageTooLarge)
	}
	mime := file.Header.Get("Content-Type")
	if !strings.HasPrefix(mime, "image/") {
		return "", iderr.InvalidProfile(iderr.ProfileReasonUnsupportedImage)
	}
	f, err := file.Open()
	if err != nil {
		return "", iderr.InvalidProfile(iderr.ProfileReasonUploadUnreadable)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", iderr.InvalidProfile(iderr.ProfileReasonUploadUnreadable)
	}

	// Old asset behind this image (if any) — deleted after the replacement lands
	// so each user keeps exactly one avatar + one cover, no orphaned blobs.
	prev, _ := c.svc.GetProfile(ctx, id.ID)
	oldAssetID := prev.AvatarAssetID
	if kind == "cover" {
		oldAssetID = prev.CoverAssetID
	}

	bearer, err := c.mgr.MintServiceToken(c.issuer, id.ID, c.audience, "", serviceTokenTTL, time.Now())
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
	if err := c.svc.SetProfileImage(ctx, id.ID, kind, view.CdnURL, view.ID); err != nil {
		return "", err
	}
	// Best-effort: drop the replaced asset (skip when dedup returned the same id).
	if oldAssetID != "" && oldAssetID != view.ID {
		_ = c.asset.Delete(ctx, bearer, oldAssetID)
	}
	return view.CdnURL, nil
}
