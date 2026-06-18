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

// AdminListAuditReq queries audit logs. Admin-only read-only endpoint.
// IdentityID matches actor OR target; all filter fields are optional.
type AdminListAuditReq struct {
	g.Meta     `path:"/api/v1/admin/audit" method:"get" tags:"admin" summary:"List audit logs (admin only)"`
	IdentityID string `json:"identityId" in:"query"`
	Event      string `json:"event" in:"query"`
	Limit      int    `json:"limit" in:"query"`
	Offset     int    `json:"offset" in:"query"`
}

// AuditEntry is one audit-log row as returned by the admin query endpoint.
type AuditEntry struct {
	ID         int64          `json:"id"`
	Event      string         `json:"event"`
	ActorID    string         `json:"actorId,omitempty"`
	TargetID   string         `json:"targetId,omitempty"`
	ActorEmail string         `json:"actorEmail,omitempty"`
	IP         string         `json:"ip,omitempty"`
	UserAgent  string         `json:"userAgent,omitempty"`
	ClientID   string         `json:"clientId,omitempty"`
	Result     string         `json:"result"`
	Detail     map[string]any `json:"detail,omitempty"`
	OccurredAt string         `json:"occurredAt"` // RFC3339
}

// AdminListAuditRes is the envelope payload for AdminListAudit.
type AdminListAuditRes struct {
	Entries []AuditEntry `json:"entries"`
}
