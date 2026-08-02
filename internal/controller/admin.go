package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	foundationauth "github.com/yueli-official/foundation/go/auth"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

// requireAdmin resolves the caller from a verified Bearer Principal when one is
// present, otherwise from the Identity session cookie, and asserts the admin
// role. It returns the caller's identity ID on success so that callers can
// inject it into ctx via actor.WithIdentity for correct audit attribution.
//
// Failure modes:
//   - no / invalid credential      → whatever Me returns (iderr.NotAuthenticated, 401)
//   - verified caller not admin    → iderr.Forbidden (403)
//
// It never reveals whether a target identity exists: a non-admin is rejected
// purely on the caller's own roles, before any target lookup or mutation.
func (c *Controller) requireAdmin(ctx context.Context) (string, error) {
	if principal, ok := foundationauth.FromContext(ctx); ok && principal != nil {
		subjectKind, _ := principal.Claim("subject_kind")
		if strings.TrimSpace(principal.Subject) == "" || subjectKind != "user" || !principal.HasRole(logic.AdminRole) {
			return "", iderr.Forbidden()
		}
		identity, err := c.svc.GetByOIDCSubject(ctx, principal.Subject)
		if err != nil || identity.Status != model.StatusActive {
			return "", iderr.Forbidden()
		}
		return identity.ID, nil
	}

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
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.role.grant", adminRoleResource(req.UserKey, req.Role),
	)
	if err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.GrantRole(ctx, target.ID, req.Role); err != nil {
		// Map the unknown-role sentinel to a 400; otherwise it leaks as a 500.
		if errors.Is(err, repo.ErrUnknownRole) {
			return nil, iderr.UnknownRole(req.Role)
		}
		return nil, err
	}
	roles, err := c.svc.GetRoles(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGrantRoleRes{Roles: roles}, nil
}

// AdminRevokeRole revokes a role from the target identity. Same admin guard and
// actor injection as AdminGrantRole.
func (c *Controller) AdminRevokeRole(ctx context.Context, req *v1.AdminRevokeRoleReq) (*v1.AdminRevokeRoleRes, error) {
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.role.revoke", adminRoleResource(req.UserKey, req.Role),
	)
	if err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.RevokeRole(ctx, target.ID, req.Role); err != nil {
		if errors.Is(err, repo.ErrUnknownRole) {
			return nil, iderr.UnknownRole(req.Role)
		}
		return nil, err
	}
	roles, err := c.svc.GetRoles(ctx, target.ID)
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
	identityID := ""
	if req.UserKey != "" {
		target, err := c.adminTarget(ctx, req.UserKey)
		if err != nil {
			return nil, err
		}
		identityID = target.ID
	}
	rows, err := c.svc.QueryAudit(ctx, repo.AuditFilter{
		IdentityID: identityID,
		Event:      req.Event,
		Limit:      req.Limit,
		Offset:     req.Offset,
	})
	if err != nil {
		return nil, err
	}
	identityIDs := make([]string, 0, len(rows)*2)
	for _, r := range rows {
		if r.ActorID != "" {
			identityIDs = append(identityIDs, r.ActorID)
		}
		if r.TargetID != "" {
			identityIDs = append(identityIDs, r.TargetID)
		}
	}
	userKeys, err := c.svc.GetUserKeysByIDs(ctx, identityIDs)
	if err != nil {
		return nil, err
	}
	out := make([]v1.AuditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAuditEntry(r, userKeys))
	}
	return &v1.AdminListAuditRes{Entries: out}, nil
}

// AdminListUsers serves the filtered, paginated admin user list.
func (c *Controller) AdminListUsers(ctx context.Context, req *v1.AdminListUsersReq) (*v1.AdminListUsersRes, error) {
	if _, err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	rows, total, err := c.svc.AdminListUsers(ctx, repo.AdminUserFilter{
		Keyword: req.Keyword,
		Status:  req.Status,
		Role:    req.Role,
		OrderBy: req.OrderBy,
		Order:   req.Order,
		Limit:   size,
		Offset:  (page - 1) * size,
	})
	if err != nil {
		return nil, err
	}
	out := make([]v1.AdminUserDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAdminUserDTO(r))
	}
	return &v1.AdminListUsersRes{List: out, Total: total}, nil
}

// AdminUserStats serves identity counts per status for the dashboard.
func (c *Controller) AdminUserStats(ctx context.Context, _ *v1.AdminUserStatsReq) (*v1.AdminUserStatsRes, error) {
	if _, err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
	counts, err := c.svc.AdminUserStats(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUserStatsRes{
		Total:    counts["total"],
		Active:   counts["active"],
		Disabled: counts["disabled"],
		Deleted:  counts["deleted"],
	}, nil
}

// AdminGetUser serves one user's admin detail.
func (c *Controller) AdminGetUser(ctx context.Context, req *v1.AdminGetUserReq) (*v1.AdminGetUserRes, error) {
	if _, err := c.requireAdmin(ctx); err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	row, err := c.svc.AdminGetUser(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminGetUserRes{User: toAdminUserDTO(row)}, nil
}

// AdminUpdateStatus sets a user's lifecycle status (ban / unban).
func (c *Controller) AdminUpdateStatus(ctx context.Context, req *v1.AdminUpdateStatusReq) (*v1.AdminUpdateStatusRes, error) {
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.status.update", adminStatusResource(req.UserKey, req.Status),
	)
	if err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.AdminSetUserStatus(ctx, target.ID, model.Status(req.Status)); err != nil {
		return nil, err
	}
	row, err := c.svc.AdminGetUser(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminUpdateStatusRes{User: toAdminUserDTO(row)}, nil
}

// AdminDeleteUser soft-deletes a user.
func (c *Controller) AdminDeleteUser(ctx context.Context, req *v1.AdminDeleteUserReq) (*v1.AdminDeleteUserRes, error) {
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.user.delete", adminIdentityResource(req.UserKey),
	)
	if err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.AdminDeleteUser(ctx, target.ID); err != nil {
		return nil, err
	}
	return &v1.AdminDeleteUserRes{}, nil
}

// AdminResetPassword overrides a user's password.
func (c *Controller) AdminResetPassword(ctx context.Context, req *v1.AdminResetPasswordReq) (*v1.AdminResetPasswordRes, error) {
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.password.reset", adminIdentityResource(req.UserKey),
	)
	if err != nil {
		return nil, err
	}
	target, err := c.adminTarget(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	if err := c.svc.AdminResetPassword(ctx, target.ID, req.NewPassword); err != nil {
		return nil, err
	}
	return &v1.AdminResetPasswordRes{}, nil
}

// AdminCreateUser provisions a new account from the admin console.
func (c *Controller) AdminCreateUser(ctx context.Context, req *v1.AdminCreateUserReq) (*v1.AdminCreateUserRes, error) {
	adminID, err := c.requireAdminAction(
		ctx, "identity.admin.user.create", adminCreateResource(req.Email, req.Roles),
	)
	if err != nil {
		return nil, err
	}
	ctx = actor.WithIdentity(ctx, adminID)
	id, err := c.svc.AdminCreateUser(ctx, logic.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	}, req.Roles)
	if err != nil {
		return nil, err
	}
	row, err := c.svc.AdminGetUser(ctx, id.ID)
	if err != nil {
		return nil, err
	}
	return &v1.AdminCreateUserRes{User: toAdminUserDTO(row)}, nil
}

// toAdminUserDTO maps a repo.AdminUserRow to the API wire type.
func toAdminUserDTO(r repo.AdminUserRow) v1.AdminUserDTO {
	roles := r.Roles
	if roles == nil {
		roles = []string{}
	}
	return v1.AdminUserDTO{
		UserKey:       r.UserKey,
		Email:         r.Email,
		EmailVerified: r.EmailVerified,
		Status:        string(r.Status),
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		DisplayName:   r.DisplayName,
		Handle:        r.Handle,
		Avatar:        mediaRef(r.AvatarMediaKey),
		Roles:         roles,
	}
}

func (c *Controller) adminTarget(ctx context.Context, userKey string) (model.Identity, error) {
	identity, err := c.svc.GetByUserKey(ctx, userKey)
	if errors.Is(err, repo.ErrIdentityMissing) {
		return model.Identity{}, iderr.IdentityNotFound()
	}
	return identity, err
}

// toAuditEntry maps a repo.AuditRow to the API wire type.
func toAuditEntry(r repo.AuditRow, userKeys map[string]string) v1.AuditEntry {
	return v1.AuditEntry{
		ID:            r.ID,
		Event:         r.Event,
		ActorUserKey:  userKeys[r.ActorID],
		TargetUserKey: userKeys[r.TargetID],
		ActorEmail:    r.ActorEmail,
		IP:            r.IP,
		UserAgent:     r.UserAgent,
		ClientID:      r.ClientID,
		RequestID:     r.RequestID,
		Result:        r.Result,
		Detail:        r.Detail,
		OccurredAt:    r.OccurredAt.Format(time.RFC3339),
	}
}
