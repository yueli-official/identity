package v1

import "github.com/gogf/gf/v2/frame/g"

type MeReq struct {
	g.Meta `path:"/api/v1/session/me" method:"get" tags:"session" summary:"Current identity"`
}
type MeRes struct {
	ID            string          `json:"id"`
	Email         string          `json:"email"`
	EmailVerified bool            `json:"emailVerified"`
	DisplayName   string          `json:"displayName"`
	Username      string          `json:"username"`
	AvatarURL     string          `json:"avatarUrl"`
	CoverURL      string          `json:"coverUrl"`
	Bio           string          `json:"bio"`
	SocialLinks   []SocialLinkDTO `json:"socialLinks"`
	Roles         []string        `json:"roles"`
}
