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

// SetPasswordReq sets an INITIAL password for an account that has none (e.g.
// OAuth-only). No current password is required; an account that already has a
// password gets identity.password_already_set and must use change-password.
type SetPasswordReq struct {
	g.Meta      `path:"/api/v1/auth/password/set" method:"post" tags:"account" summary:"Set an initial password"`
	NewPassword string `json:"newPassword" v:"required#new password required"`
}

type SetPasswordRes struct{}

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

// ── Credentials (login methods: password + bound oauth) ─────────────────────

type CredentialsReq struct {
	g.Meta `path:"/api/v1/session/credentials" method:"get" tags:"account" summary:"List own login credentials"`
}

type OAuthCredentialDTO struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
}

type CredentialsRes struct {
	HasPassword bool                 `json:"hasPassword"`
	OAuth       []OAuthCredentialDTO `json:"oauth"`
}

type UnbindCredentialReq struct {
	g.Meta   `path:"/api/v1/session/credentials/{provider}" method:"delete" tags:"account" summary:"Unbind an oauth credential"`
	Provider string `json:"provider" in:"path"`
}

type UnbindCredentialRes struct{}
