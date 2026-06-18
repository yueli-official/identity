package controller

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// requireAdmin resolves the caller from the session cookie and asserts the admin
// role. It returns the caller's identity ID on success so that callers can
// inject it into ctx via actor.WithIdentity for correct audit attribution.
//
// Failure modes:
//   - no / invalid session cookie  → whatever Me returns (iderr.NotAuthenticated, 401)
//   - authenticated but not admin  → iderr.Forbidden (403)
//
// It never reveals whether a target identity exists: a non-admin is rejected
// purely on the caller's own roles, before any target lookup or mutation.
func (c *Controller) requireAdmin(ctx context.Context) (string, error) {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return "", err // not authenticated → coded 401 from Me (iderr.NotAuthenticated)
	}
	roles, err := c.svc.GetRoles(ctx, id.ID)
	if err != nil {
		return "", err
	}
	for _, role := range roles {
		if role == logic.AdminRole {
			return id.ID, nil
		}
	}
	return "", iderr.Forbidden()
}

// AdminGrantRole grants a role to the target identity. The admin guard runs
// first; a non-admin / unauthenticated caller never reaches GrantRole.
// The resolved admin identity is injected into ctx so the audit record
// attributes the mutation to the correct actor.
func (c *Controller) AdminGrantRole(ctx context.Context, req *v1.AdminGrantRoleReq) (*v1.AdminGrantRoleRes, error) {
	adminID, err := c.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.GrantRole(ctx, req.IdentityID, req.Role); err != nil {
		// Map the unknown-role sentinel to a 400; otherwise it leaks as a 500.
		if errors.Is(err, repo.ErrUnknownRole) {
			return nil, iderr.UnknownRole(req.Role)
		}
		return nil, err
	}
	roles, err := c.svc.GetRoles(ctx, req.IdentityID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGrantRoleRes{Roles: roles}, nil
}

// AdminRevokeRole revokes a role from the target identity. Same admin guard and
// actor injection as AdminGrantRole.
func (c *Controller) AdminRevokeRole(ctx context.Context, req *v1.AdminRevokeRoleReq) (*v1.AdminRevokeRoleRes, error) {
	adminID, err := c.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.RevokeRole(ctx, req.IdentityID, req.Role); err != nil {
		if errors.Is(err, repo.ErrUnknownRole) {
			return nil, iderr.UnknownRole(req.Role)
		}
		return nil, err
	}
	roles, err := c.svc.GetRoles(ctx, req.IdentityID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminRevokeRoleRes{Roles: roles}, nil
}

// AdminListAudit returns audit log entries filtered by the query parameters.
// It is read-only; the admin guard still runs to restrict access.
func (c *Controller) AdminListAudit(ctx context.Context, req *v1.AdminListAuditReq) (*v1.AdminListAuditRes, error) {
	if _, err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := c.svc.QueryAudit(ctx, repo.AuditFilter{
		IdentityID: req.IdentityID,
		Event:      req.Event,
		Limit:      req.Limit,
		Offset:     req.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]v1.AuditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAuditEntry(r))
	}
	return &v1.AdminListAuditRes{Entries: out}, nil
}

// toAuditEntry maps a repo.AuditRow to the API wire type.
func toAuditEntry(r repo.AuditRow) v1.AuditEntry {
	return v1.AuditEntry{
		ID:         r.ID,
		Event:      r.Event,
		ActorID:    r.ActorID,
		TargetID:   r.TargetID,
		ActorEmail: r.ActorEmail,
		IP:         r.IP,
		UserAgent:  r.UserAgent,
		ClientID:   r.ClientID,
		RequestID:  r.RequestID,
		Result:     r.Result,
		Detail:     r.Detail,
		OccurredAt: r.OccurredAt.Format(time.RFC3339),
	}
}
