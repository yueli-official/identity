package controller

import (
	"context"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
)

func (c *Controller) Me(ctx context.Context, _ *v1.MeReq) (*v1.MeRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	id, err := c.svc.Me(ctx, sid)
	if err != nil {
		return nil, err
	}
	return &v1.MeRes{ID: id.ID, Email: id.Email, EmailVerified: id.EmailVerified}, nil
}
