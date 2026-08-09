package v1

import "github.com/gogf/gf/v2/frame/g"

// AdminGrantRoleReq grants a role to an identity. Admin-only: the controller
// asserts the caller holds the "admin" role before any state change.
type AdminGrantRoleReq struct {
	g.Meta  `path:"/api/v1/admin/users/{userKey}/roles" method:"post" tags:"admin" summary:"Grant a role (admin only)"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
	Role    string `json:"role" v:"required"`
}
type AdminGrantRoleRes struct {
	Roles []string `json:"roles"`
}

// AdminRevokeRoleReq revokes a role from an identity. Admin-only (same guard).
type AdminRevokeRoleReq struct {
	g.Meta  `path:"/api/v1/admin/users/{userKey}/roles/{role}" method:"delete" tags:"admin" summary:"Revoke a role (admin only)"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
	Role    string `json:"role" in:"path"`
}
type AdminRevokeRoleRes struct {
	Roles []string `json:"roles"`
}

// AdminListAuditReq queries audit logs. Admin-only read-only endpoint.
// UserKey matches actor OR target; all filter fields are optional.
type AdminListAuditReq struct {
	g.Meta  `path:"/api/v1/admin/audit" method:"get" tags:"admin" summary:"List audit logs (admin only)"`
	UserKey string `json:"userKey" in:"query" v:"regex:^$|^[1-9A-HJ-NP-Za-km-z]{8}$"`
	Event   string `json:"event" in:"query"`
	Limit   int    `json:"limit" in:"query"`
	Offset  int    `json:"offset" in:"query"`
}

// AuditEntry is one audit-log row as returned by the admin query endpoint.
type AuditEntry struct {
	ID            int64          `json:"id"`
	Event         string         `json:"event"`
	ActorUserKey  string         `json:"actorUserKey,omitempty"`
	TargetUserKey string         `json:"targetUserKey,omitempty"`
	ActorEmail    string         `json:"actorEmail,omitempty"`
	IP            string         `json:"ip,omitempty"`
	UserAgent     string         `json:"userAgent,omitempty"`
	ClientID      string         `json:"clientId,omitempty"`
	RequestID     string         `json:"requestId,omitempty"`
	Result        string         `json:"result"`
	Detail        map[string]any `json:"detail,omitempty"`
	OccurredAt    string         `json:"occurredAt"` // RFC3339
}

// AdminListAuditRes is the envelope payload for AdminListAudit.
type AdminListAuditRes struct {
	Entries []AuditEntry `json:"entries"`
}

// ── Admin user management ────────────────────────────────────────────────────

// AdminUserDTO is one user as surfaced to the admin console (list + detail):
// the identity joined with its profile display fields and role slugs.
type AdminUserDTO struct {
	UserKey       string    `json:"userKey"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"emailVerified"`
	Status        string    `json:"status"`    // active | disabled | deleted
	CreatedAt     string    `json:"createdAt"` // RFC3339
	DisplayName   string    `json:"displayName"`
	Handle        string    `json:"handle"`
	Avatar        *MediaRef `json:"avatar,omitempty"`
	Roles         []string  `json:"roles"`
}

// AdminListUsersReq is the filtered, paginated user list (admin only).
type AdminListUsersReq struct {
	g.Meta  `path:"/api/v1/admin/users" method:"get" tags:"admin" summary:"List users (admin only)"`
	Keyword string `json:"keyword" in:"query"`
	Status  string `json:"status" in:"query"`  // active|disabled|deleted; "" = non-deleted
	Role    string `json:"role" in:"query"`    // role slug; "" = any
	OrderBy string `json:"orderBy" in:"query"` // created_at|display_name
	Order   string `json:"order" in:"query"`   // asc|desc
	Page    int    `json:"page" in:"query"`
	Size    int    `json:"size" in:"query"`
}
type AdminListUsersRes struct {
	List  []AdminUserDTO `json:"list"`
	Total int            `json:"total"`
}

// AdminUserStatsReq returns identity counts per status for the dashboard.
type AdminUserStatsReq struct {
	g.Meta `path:"/api/v1/admin/users/stats" method:"get" tags:"admin" summary:"User counts by status (admin only)"`
}
type AdminUserStatsRes struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Disabled int `json:"disabled"`
	Deleted  int `json:"deleted"`
}

// AdminGetUserReq returns one user's admin detail.
type AdminGetUserReq struct {
	g.Meta  `path:"/api/v1/admin/users/{userKey}" method:"get" tags:"admin" summary:"Get a user (admin only)"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
}
type AdminGetUserRes struct {
	User AdminUserDTO `json:"user"`
}

// AdminUpdateStatusReq sets a user's lifecycle status (ban=disabled, unban=active).
type AdminUpdateStatusReq struct {
	g.Meta  `path:"/api/v1/admin/users/{userKey}/status" method:"put" tags:"admin" summary:"Set user status (admin only)"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
	Status  string `json:"status" v:"required"`
}
type AdminUpdateStatusRes struct {
	User AdminUserDTO `json:"user"`
}

// AdminDeleteUserReq soft-deletes a user (status='deleted').
type AdminDeleteUserReq struct {
	g.Meta  `path:"/api/v1/admin/users/{userKey}" method:"delete" tags:"admin" summary:"Delete a user (admin only)"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
}
type AdminDeleteUserRes struct{}

// AdminResetPasswordReq overrides a user's password (admin override, no current-pw check).
type AdminResetPasswordReq struct {
	g.Meta      `path:"/api/v1/admin/users/{userKey}/password" method:"post" tags:"admin" summary:"Reset a user's password (admin only)"`
	UserKey     string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
	NewPassword string `json:"newPassword" v:"required"`
}
type AdminResetPasswordRes struct{}

// AdminCreateUserReq provisions a new account from the admin console.
type AdminCreateUserReq struct {
	g.Meta      `path:"/api/v1/admin/users" method:"post" tags:"admin" summary:"Create a user (admin only)"`
	Email       string   `json:"email" v:"required"`
	Password    string   `json:"password" v:"required"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
}
type AdminCreateUserRes struct {
	User AdminUserDTO `json:"user"`
}
