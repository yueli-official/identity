package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminGrantRoleReq grants a role to an identity. Admin-only: the controller
// asserts the caller holds the "admin" role before any state change.
type AdminGrantRoleReq struct {
	g.Meta     `path:"/api/v1/admin/identities/{identityId}/roles" method:"post" tags:"admin" summary:"Grant a role (admin only)"`
	IdentityID string `json:"identityId" in:"path"`
	Role       string `json:"role" v:"required"`
}
type AdminGrantRoleRes struct {
	Roles []string `json:"roles"`
}

// AdminRevokeRoleReq revokes a role from an identity. Admin-only (same guard).
type AdminRevokeRoleReq struct {
	g.Meta     `path:"/api/v1/admin/identities/{identityId}/roles/{role}" method:"delete" tags:"admin" summary:"Revoke a role (admin only)"`
	IdentityID string `json:"identityId" in:"path"`
	Role       string `json:"role" in:"path"`
}
type AdminRevokeRoleRes struct {
	Roles []string `json:"roles"`
}
