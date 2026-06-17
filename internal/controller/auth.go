package controller

import (
	"context"
	"net/http"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/logic"
)

const sessionCookie = "id_session"

// Controller handles HTTP requests for the identity service.
type Controller struct {
	svc          *logic.Service
	secureCookie bool
}

// New builds the controller. secureCookie should be true in production (HTTPS);
// tests/local HTTP use false so the session cookie round-trips over plain HTTP.
func New(svc *logic.Service, secureCookie bool) *Controller {
	return &Controller{svc: svc, secureCookie: secureCookie}
}

func (c *Controller) Register(ctx context.Context, req *v1.RegisterReq) (*v1.RegisterRes, error) {
	id, err := c.svc.Register(ctx, logic.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RegisterRes{ID: id.ID, Email: id.Email}, nil
}

func (c *Controller) Login(ctx context.Context, req *v1.LoginReq) (*v1.LoginRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	out, err := c.svc.Login(ctx, logic.LoginInput{
		Email: req.Email, Password: req.Password,
		UserAgent: r.UserAgent(), IP: r.GetClientIp(),
	})
	if err != nil {
		return nil, err
	}
	c.setSessionCookie(r, out.SessionID)
	return &v1.LoginRes{ID: out.Identity.ID, Email: out.Identity.Email}, nil
}

func (c *Controller) Logout(ctx context.Context, _ *v1.LogoutReq) (*v1.LogoutRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	if sid != "" {
		if err := c.svc.Logout(ctx, sid); err != nil {
			return nil, err
		}
	}
	r.Cookie.Remove(sessionCookie)
	return &v1.LogoutRes{}, nil
}

func (c *Controller) setSessionCookie(r *ghttp.Request, sid string) {
	r.Cookie.SetHttpCookie(&http.Cookie{
		Name: sessionCookie, Value: sid, Path: "/",
		HttpOnly: true, Secure: c.secureCookie, SameSite: http.SameSiteLaxMode,
	})
}
