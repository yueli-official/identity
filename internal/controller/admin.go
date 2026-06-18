package controller

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// requireAdmin resolves the caller from the session cookie and asserts the admin
// role. It is the single gate for every admin-only endpoint and MUST be called
// before any state change. Failure modes:
//   - no / invalid session cookie  → whatever Me returns (iderr.NotAuthenticated, 401)
//   - authenticated but not admin  → iderr.Forbidden (403)
//
// It never reveals whether a target identity exists: a non-admin is rejected
// purely on the caller's own roles, before any target lookup or mutation.
func (c *Controller) requireAdmin(ctx context.Context) error {
	r := ghttp.RequestFromCtx(ctx)
	id, err := c.svc.Me(ctx, r.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return err // not authenticated → coded 401 from Me (iderr.NotAuthenticated)
	}
	roles, err := c.svc.GetRoles(ctx, id.ID)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if role == logic.AdminRole {
			return nil
		}
	}
	return iderr.Forbidden()
}

// AdminGrantRole grants a role to the target identity. The admin guard runs
// first; a non-admin / unauthenticated caller never reaches GrantRole.
func (c *Controller) AdminGrantRole(ctx context.Context, req *v1.AdminGrantRoleReq) (*v1.AdminGrantRoleRes, error) {
	if err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
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

// AdminRevokeRole revokes a role from the target identity. Same admin guard.
func (c *Controller) AdminRevokeRole(ctx context.Context, req *v1.AdminRevokeRoleReq) (*v1.AdminRevokeRoleRes, error) {
	if err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
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
