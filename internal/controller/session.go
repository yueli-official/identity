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
	p, _ := c.svc.GetProfile(ctx, id.ID) // empty profile is acceptable
	return &v1.MeRes{
		ID:            id.ID,
		Email:         id.Email,
		EmailVerified: id.EmailVerified,
		DisplayName:   p.DisplayName,
		Username:      p.Username,
		AvatarURL:     p.AvatarURL,
		CoverURL:      p.CoverURL,
		Bio:           p.Bio,
		SocialLinks:   socialToDTO(p.SocialLinks),
	}, nil
}
