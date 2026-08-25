package v1

import "github.com/gogf/gf/v2/frame/g"

// ── Profile edit (account self-management) ──────────────────────────────────

// SocialLinkDTO is one labelled external link on a profile.
type SocialLinkDTO struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type UpdateProfileReq struct {
	g.Meta      `path:"/api/v1/session/profile" method:"put" tags:"account" summary:"Update own profile"`
	DisplayName string `json:"displayName" v:"required#display name required"`
	Handle      string `json:"handle"`
	Bio         string `json:"bio"`
	Locale      string `json:"locale"`
}

type UpdateProfileRes struct {
	DisplayName string          `json:"displayName"`
	Handle      string          `json:"handle"`
	Avatar      *MediaRef       `json:"avatar,omitempty"`
	Cover       *MediaRef       `json:"cover,omitempty"`
	Bio         string          `json:"bio"`
	SocialLinks []SocialLinkDTO `json:"socialLinks"`
	Locale      string          `json:"locale"`
}

type UpdateSocialLinksReq struct {
	g.Meta      `path:"/api/v1/session/profile/social-links" method:"put" tags:"account" summary:"Replace own social links"`
	SocialLinks []SocialLinkDTO `json:"socialLinks"`
}

type UpdateSocialLinksRes struct {
	SocialLinks []SocialLinkDTO `json:"socialLinks"`
}

// ── Change password (authenticated; not the token-based reset) ──────────────

type ChangePasswordReq struct {
	g.Meta          `path:"/api/v1/auth/password/change" method:"post" tags:"account" summary:"Change own password"`
	CurrentPassword string `json:"currentPassword" v:"required#current password required"`
	NewPassword     string `json:"newPassword" v:"required#new password required"`
}

type ChangePasswordRes struct{}

type ReauthenticateReq struct {
	g.Meta   `path:"/api/v1/auth/reauthenticate" method:"post" tags:"account" summary:"Refresh current session authentication with the account password"`
	Password string `json:"password" v:"required#password required"`
}

type ReauthenticateRes struct {
	AuthenticatedAt string `json:"authenticatedAt"`
}

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
	Limit  int `json:"limit" in:"query"`
	Offset int `json:"offset" in:"query"`
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
	Current *SessionEntry  `json:"current,omitempty"`
	Entries []SessionEntry `json:"entries"`
	Total   int            `json:"total"`
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

type LogoutOthersReq struct {
	g.Meta `path:"/api/v1/auth/logout-others" method:"post" tags:"account" summary:"Log out other sessions"`
}

type LogoutOthersRes struct{}

// ── Credentials (login methods: password + bound oauth) ─────────────────────

type CredentialsReq struct {
	g.Meta `path:"/api/v1/session/credentials" method:"get" tags:"account" summary:"List own login credentials"`
}

type OAuthCredentialDTO struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
}

type CredentialsRes struct {
	HasPassword  bool                 `json:"hasPassword"`
	OAuth        []OAuthCredentialDTO `json:"oauth"`
	PasskeyCount int                  `json:"passkeyCount"`
}

type UnbindCredentialReq struct {
	g.Meta   `path:"/api/v1/session/credentials/{provider}" method:"delete" tags:"account" summary:"Unbind an oauth credential"`
	Provider string `json:"provider" in:"path"`
}

type UnbindCredentialRes struct{}
