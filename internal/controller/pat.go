package controller

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/iderr"
)

// patRFC3339 formats a *time.Time as RFC3339, returning "" for nil.
func patRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// CreatePAT creates a new Personal Access Token for the authenticated caller.
// The plaintext token is returned once in the response and never stored again.
func (c *Controller) CreatePAT(ctx context.Context, req *v1.CreatePATReq) (*v1.CreatePATRes, error) {
	r := g.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, iderr.NotAuthenticated()
	}

	plaintext, row, err := c.svc.CreatePAT(ctx, id.ID, req.Name, req.Scopes, req.ExpiresInDays)
	if err != nil {
		return nil, err
	}

	return &v1.CreatePATRes{
		ID:          row.ID,
		Name:        row.Name,
		Token:       plaintext,
		TokenPrefix: row.TokenPrefix,
		Scopes:      row.Scopes,
		ExpiresAt:   patRFC3339(row.ExpiresAt),
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
	}, nil
}

// ListPAT returns all Personal Access Tokens belonging to the authenticated caller.
// Token hash and plaintext are never included in the response.
func (c *Controller) ListPAT(ctx context.Context, _ *v1.ListPATReq) (*v1.ListPATRes, error) {
	r := g.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, iderr.NotAuthenticated()
	}

	rows, err := c.svc.ListPAT(ctx, id.ID)
	if err != nil {
		return nil, err
	}

	entries := make([]v1.PATEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, v1.PATEntry{
			ID:          row.ID,
			Name:        row.Name,
			TokenPrefix: row.TokenPrefix,
			Scopes:      row.Scopes,
			ExpiresAt:   patRFC3339(row.ExpiresAt),
			LastUsedAt:  patRFC3339(row.LastUsedAt),
			CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		})
	}

	return &v1.ListPATRes{Entries: entries}, nil
}

// RevokePAT deletes the identified PAT, enforcing ownership — only the token's
// owner can revoke it.
func (c *Controller) RevokePAT(ctx context.Context, req *v1.RevokePATReq) (*v1.RevokePATRes, error) {
	r := g.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, iderr.NotAuthenticated()
	}

	if err := c.svc.RevokePAT(ctx, id.ID, req.ID); err != nil {
		return nil, err
	}
	return &v1.RevokePATRes{}, nil
}

// VerifyPAT validates a Bearer token from the Authorization header. This
// endpoint is public — it does not require a session cookie and is intended
// for resource servers that need to validate a PAT on behalf of a caller.
func (c *Controller) VerifyPAT(ctx context.Context, _ *v1.VerifyPATReq) (*v1.VerifyPATRes, error) {
	r := g.RequestFromCtx(ctx)
	authz := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authz, "Bearer ")
	if token == authz || token == "" {
		return nil, iderr.PATInvalid()
	}

	res, err := c.svc.VerifyPAT(ctx, token)
	if err != nil {
		return nil, err
	}

	return &v1.VerifyPATRes{
		IdentityID: res.IdentityID,
		Scopes:     res.Scopes,
		ExpiresAt:  patRFC3339(res.ExpiresAt),
	}, nil
}
