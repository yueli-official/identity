package v1

import "github.com/gogf/gf/v2/frame/g"

// ── Profile edit (account self-management) ──────────────────────────────────

type UpdateProfileReq struct {
	g.Meta      `path:"/api/v1/session/profile" method:"put" tags:"account" summary:"Update own profile"`
	DisplayName string `json:"displayName" v:"required#display name required"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatarUrl"`
	Locale      string `json:"locale"`
}

type UpdateProfileRes struct {
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatarUrl"`
	Locale      string `json:"locale"`
}

// ── Change password (authenticated; not the token-based reset) ──────────────

type ChangePasswordReq struct {
	g.Meta          `path:"/api/v1/auth/password/change" method:"post" tags:"account" summary:"Change own password"`
	CurrentPassword string `json:"currentPassword" v:"required#current password required"`
	NewPassword     string `json:"newPassword" v:"required#new password required"`
}

type ChangePasswordRes struct{}

// ── Session list / revoke (account-center device management) ────────────────

type SessionListReq struct {
	g.Meta `path:"/api/v1/session/list" method:"get" tags:"account" summary:"List own login sessions"`
}

type SessionEntry struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	LastSeen  string `json:"lastSeen"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Current   bool   `json:"current"`
}

type SessionListRes struct {
	Entries []SessionEntry `json:"entries"`
}

type RevokeSessionReq struct {
	g.Meta `path:"/api/v1/session/{id}" method:"delete" tags:"account" summary:"Revoke one session"`
	ID     string `json:"id" in:"path"`
}

type RevokeSessionRes struct{}

type LogoutAllReq struct {
	g.Meta `path:"/api/v1/auth/logout-all" method:"post" tags:"account" summary:"Log out all sessions"`
}

type LogoutAllRes struct{}
