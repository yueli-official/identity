package v1

import "github.com/gogf/gf/v2/frame/g"

type MeReq struct {
	g.Meta `path:"/api/v1/session/me" method:"get" tags:"session" summary:"Current identity"`
}
type MeRes struct {
	UserKey       string          `json:"userKey"`
	Email         string          `json:"email"`
	EmailVerified bool            `json:"emailVerified"`
	DisplayName   string          `json:"displayName"`
	Handle        string          `json:"handle"`
	Avatar        *MediaRef       `json:"avatar,omitempty"`
	Cover         *MediaRef       `json:"cover,omitempty"`
	Bio           string          `json:"bio"`
	SocialLinks   []SocialLinkDTO `json:"socialLinks"`
	Roles         []string        `json:"roles"`
}
