package v1

import "github.com/gogf/gf/v2/frame/g"

type CreatePATReq struct {
	g.Meta        `path:"/api/v1/pat" method:"post" tags:"pat" summary:"Create a personal access token"`
	Name          string   `json:"name" v:"required"`
	Scopes        []string `json:"scopes" v:"required"`
	ExpiresInDays int      `json:"expiresInDays"` // 0 = never
}
type CreatePATRes struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token"` // PLAINTEXT — shown once, never again
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expiresAt,omitempty"` // RFC3339 or ""
	CreatedAt   string   `json:"createdAt"`
}

type ListPATReq struct {
	g.Meta `path:"/api/v1/pat" method:"get" tags:"pat" summary:"List my personal access tokens"`
}
type PATEntry struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"tokenPrefix"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
	LastUsedAt  string   `json:"lastUsedAt,omitempty"`
	CreatedAt   string   `json:"createdAt"`
}
type ListPATRes struct {
	Entries []PATEntry `json:"entries"`
}

type RevokePATReq struct {
	g.Meta `path:"/api/v1/pat/{id}" method:"delete" tags:"pat" summary:"Revoke a personal access token"`
	ID     int64 `json:"id" in:"path"`
}
type RevokePATRes struct{}

type VerifyPATReq struct {
	g.Meta `path:"/api/v1/pat/verify" method:"post" tags:"pat" summary:"Verify a PAT (resource servers)"`
	// token comes from the Authorization: Bearer header, not the body
}
type VerifyPATRes struct {
	UserKey   string   `json:"userKey"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expiresAt,omitempty"`
}

type PATScopeCatalogReq struct {
	g.Meta `path:"/api/v1/pat/scopes" method:"get" tags:"pat" summary:"List supported personal access token scopes"`
}

type PATScopeEntry struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PATScopeCatalogRes struct {
	Entries []PATScopeEntry `json:"entries"`
}

type PATUserInfoReq struct {
	g.Meta `path:"/api/v1/pat/userinfo" method:"get" tags:"pat" summary:"Read own account information with a PAT"`
}

type PATUserInfoRes struct {
	UserKey       string          `json:"userKey"`
	Email         string          `json:"email,omitempty"`
	EmailVerified *bool           `json:"emailVerified,omitempty"`
	DisplayName   string          `json:"displayName,omitempty"`
	Handle        string          `json:"handle,omitempty"`
	Bio           string          `json:"bio,omitempty"`
	Avatar        *MediaRef       `json:"avatar,omitempty"`
	Cover         *MediaRef       `json:"cover,omitempty"`
	SocialLinks   []SocialLinkDTO `json:"socialLinks,omitempty"`
}
