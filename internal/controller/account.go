package controller

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/logic"
)

// UpdateProfile updates the logged-in caller's own profile.
func (c *Controller) UpdateProfile(ctx context.Context, req *v1.UpdateProfileReq) (*v1.UpdateProfileRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	p, err := c.svc.UpdateProfile(ctx, id.ID, logic.ProfileUpdate{
		DisplayName: req.DisplayName, Username: req.Username, AvatarURL: req.AvatarURL, Locale: req.Locale,
	})
	if err != nil {
		return nil, err
	}
	return &v1.UpdateProfileRes{
		DisplayName: p.DisplayName, Username: p.Username, AvatarURL: p.AvatarURL, Locale: p.Locale,
	}, nil
}

// ChangePassword changes the caller's password (verifying the current one) and
// revokes their other sessions, keeping the current one.
func (c *Controller) ChangePassword(ctx context.Context, req *v1.ChangePasswordReq) (*v1.ChangePasswordRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	id, err := c.svc.Me(ctx, sid)
	if err != nil {
		return nil, err
	}
	if err := c.svc.ChangePassword(ctx, id.ID, sid, req.CurrentPassword, req.NewPassword); err != nil {
		return nil, err
	}
	return &v1.ChangePasswordRes{}, nil
}

// SessionList returns the caller's active login sessions, flagging the current.
func (c *Controller) SessionList(ctx context.Context, _ *v1.SessionListReq) (*v1.SessionListRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	id, err := c.svc.Me(ctx, sid)
	if err != nil {
		return nil, err
	}
	sessions, err := c.svc.ListSessions(ctx, id.ID)
	if err != nil {
		return nil, err
	}
	entries := make([]v1.SessionEntry, 0, len(sessions))
	for _, s := range sessions {
		entries = append(entries, v1.SessionEntry{
			ID:        s.ID,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
			LastSeen:  s.LastSeen.Format(time.RFC3339),
			IP:        s.IP,
			UserAgent: s.UserAgent,
			Current:   s.ID == sid,
		})
	}
	return &v1.SessionListRes{Entries: entries}, nil
}

// RevokeSession revokes one of the caller's own sessions (ownership-checked).
func (c *Controller) RevokeSession(ctx context.Context, req *v1.RevokeSessionReq) (*v1.RevokeSessionRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	if err := c.svc.RevokeSession(ctx, id.ID, req.ID); err != nil {
		return nil, err
	}
	return &v1.RevokeSessionRes{}, nil
}

// Credentials lists the caller's login methods (password + bound oauth).
func (c *Controller) Credentials(ctx context.Context, _ *v1.CredentialsReq) (*v1.CredentialsRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	cs, err := c.svc.ListCredentials(ctx, id.ID)
	if err != nil {
		return nil, err
	}
	oauth := make([]v1.OAuthCredentialDTO, 0, len(cs.OAuth))
	for _, o := range cs.OAuth {
		oauth = append(oauth, v1.OAuthCredentialDTO{Provider: o.Provider, Email: o.Email})
	}
	return &v1.CredentialsRes{HasPassword: cs.HasPassword, OAuth: oauth}, nil
}

// UnbindCredential removes one of the caller's oauth credentials (last-credential guarded).
func (c *Controller) UnbindCredential(ctx context.Context, req *v1.UnbindCredentialReq) (*v1.UnbindCredentialRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	if err := c.svc.UnbindCredential(ctx, id.ID, req.Provider); err != nil {
		return nil, err
	}
	return &v1.UnbindCredentialRes{}, nil
}

// LogoutAll clears every session of the caller and revokes all refresh tokens.
func (c *Controller) LogoutAll(ctx context.Context, _ *v1.LogoutAllReq) (*v1.LogoutAllRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	if err := c.svc.LogoutAll(ctx, id.ID); err != nil {
		return nil, err
	}
	r.Cookie.Remove(sessionCookie)
	return &v1.LogoutAllRes{}, nil
}
