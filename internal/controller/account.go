package controller

import (
	"context"
	"sort"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/logic"
)

// UpdateProfile updates the logged-in caller's own profile.
func (c *Controller) UpdateProfile(ctx context.Context, req *v1.UpdateProfileReq) (*v1.UpdateProfileRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	p, err := c.svc.UpdateProfile(ctx, id.ID, logic.ProfileUpdate{
		DisplayName: req.DisplayName, Handle: req.Handle, Bio: req.Bio, Locale: req.Locale,
	})
	if err != nil {
		return nil, err
	}
	return &v1.UpdateProfileRes{
		DisplayName: p.DisplayName, Handle: p.Handle, Avatar: mediaRef(p.AvatarMediaKey),
		Cover: mediaRef(p.CoverMediaKey), Bio: p.Bio, SocialLinks: socialToDTO(p.SocialLinks),
		Locale: p.Locale,
	}, nil
}

func (c *Controller) UpdateSocialLinks(ctx context.Context, req *v1.UpdateSocialLinksReq) (*v1.UpdateSocialLinksRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	links, err := c.svc.UpdateSocialLinks(ctx, id.ID, socialFromDTO(req.SocialLinks))
	if err != nil {
		return nil, err
	}
	return &v1.UpdateSocialLinksRes{SocialLinks: socialToDTO(links)}, nil
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

// SetPassword sets an initial password for the caller's account when it has none
// (e.g. an OAuth-only account adding a password before unbinding its OAuth login).
func (c *Controller) SetPassword(ctx context.Context, req *v1.SetPasswordReq) (*v1.SetPasswordRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	if err := c.svc.SetPassword(ctx, id.ID, req.NewPassword); err != nil {
		return nil, err
	}
	return &v1.SetPasswordRes{}, nil
}

// SessionList returns the caller's active login sessions, flagging the current.
func (c *Controller) SessionList(ctx context.Context, req *v1.SessionListReq) (*v1.SessionListRes, error) {
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
	sort.SliceStable(sessions, func(left, right int) bool {
		return sessions[left].CreatedAt.After(sessions[right].CreatedAt)
	})
	entries := make([]v1.SessionEntry, 0, len(sessions))
	var current *v1.SessionEntry
	for _, s := range sessions {
		entry := v1.SessionEntry{
			ID:        s.ID,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
			LastSeen:  s.LastSeen.Format(time.RFC3339),
			IP:        s.IP,
			UserAgent: s.UserAgent,
			Current:   s.ID == sid,
		}
		if entry.Current {
			copy := entry
			current = &copy
			continue
		}
		entries = append(entries, entry)
	}
	total := len(entries)
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		entries = []v1.SessionEntry{}
	} else {
		entries = entries[offset:]
		if len(entries) > limit {
			entries = entries[:limit]
		}
	}
	return &v1.SessionListRes{Current: current, Entries: entries, Total: total}, nil
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
	return &v1.CredentialsRes{
		HasPassword: cs.HasPassword, OAuth: oauth, PasskeyCount: cs.PasskeyCount,
	}, nil
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

// LogoutOthers clears every other session while preserving the browser that
// initiated the action. It is deliberately different from LogoutAll: the
// account-center action must not contradict its "other sessions" label.
func (c *Controller) LogoutOthers(ctx context.Context, _ *v1.LogoutOthersReq) (*v1.LogoutOthersRes, error) {
	r := ghttp.RequestFromCtx(ctx)
	sid := r.Cookie.Get(sessionCookie, "").String()
	id, err := c.svc.Me(ctx, sid)
	if err != nil {
		return nil, err
	}
	if err := c.svc.LogoutOtherSessions(ctx, id.ID, sid); err != nil {
		return nil, err
	}
	return &v1.LogoutOthersRes{}, nil
}
