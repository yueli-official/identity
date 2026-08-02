package controller

import (
	"context"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/yueli-official/identity/api/v1"
)

func (c *Controller) Me(ctx context.Context, _ *v1.MeReq) (*v1.MeRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	id, err := c.svc.Me(ctx, sid)
	if err != nil {
		return nil, err
	}
	p, _ := c.svc.GetProfile(ctx, id.ID)   // empty profile is acceptable
	roles, _ := c.svc.GetRoles(ctx, id.ID) // empty roles acceptable
	if roles == nil {
		roles = []string{}
	}
	return &v1.MeRes{
		UserKey:       id.UserKey,
		Email:         id.Email,
		EmailVerified: id.EmailVerified,
		DisplayName:   p.DisplayName,
		Handle:        p.Handle,
		Avatar:        mediaRef(p.AvatarMediaKey),
		Cover:         mediaRef(p.CoverMediaKey),
		Bio:           p.Bio,
		SocialLinks:   socialToDTO(p.SocialLinks),
		Roles:         roles,
	}, nil
}
