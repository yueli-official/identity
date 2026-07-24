package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/yueli-official/foundation/go/privacy"

	v1 "platform/services/identity/api/v1"
)

func (c *Controller) OpenErasure(ctx context.Context, req *v1.OpenErasureReq) (*v1.OpenErasureRes, error) {
	if c.privacy == nil {
		return nil, errors.New("identity privacy coordinator is not configured")
	}
	requestedAt, err := time.Parse(time.RFC3339, req.RequestedAt)
	if err != nil {
		return nil, err
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	identity, err := c.svc.Me(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	sessionDigest := sha256.Sum256([]byte(sessionID))
	view, err := c.privacy.OpenErasure(
		ctx, identity.ID, identity.Email, privacy.IdempotencyKey(req.IdempotencyKey), req.StatusToken, requestedAt,
		privacy.VerificationEvidence{
			VerifiedAt: requestedAt, Method: "active_identity_session",
			Assurance: "single_factor", VerificationRef: hex.EncodeToString(sessionDigest[:]),
		},
	)
	if err != nil {
		return nil, err
	}
	return &v1.OpenErasureRes{Request: view}, nil
}

func (c *Controller) PrivacyRequestStatus(
	ctx context.Context, req *v1.PrivacyRequestStatusReq,
) (*v1.PrivacyRequestStatusRes, error) {
	if c.privacy == nil {
		return nil, errors.New("identity privacy coordinator is not configured")
	}
	view, err := c.privacy.GetByToken(ctx, req.StatusToken, privacy.RightsRequestID(req.ID))
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyRequestStatusRes{Request: view}, nil
}
