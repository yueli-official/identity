package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/yueli-official/foundation/go/privacy"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/authentication"
)

const privacyRecentAuthentication = 5 * time.Minute

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
	session, identity, err := c.svc.RequireAuthentication(ctx, sessionID, authentication.Requirement{
		FreshWithin:     privacyRecentAuthentication,
		MinimumLevel:    authentication.LevelAAL1,
		MinimumProfile:  authentication.ProfileBaseline,
		RecoveryAllowed: false,
	})
	if err != nil {
		return nil, err
	}
	view, err := c.privacy.OpenErasure(
		ctx, identity.ID, identity.UserKey, identity.Email, privacy.IdempotencyKey(req.IdempotencyKey), req.StatusToken, requestedAt,
		privacy.VerificationEvidence{
			VerifiedAt:      session.Authentication.AuthenticatedAt,
			Method:          strings.Join(authentication.MethodStrings(session.Authentication.Methods), "+"),
			Assurance:       string(session.Authentication.Profile),
			VerificationRef: session.Authentication.EventID,
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
