package v1

import "github.com/gogf/gf/v2/frame/g"

// ── Public profiles (cross-user resolution; unauthenticated, public subset) ──
// Consumer sites (blog author pages, bylines) resolve any user's display data
// here. Never returns email / roles / status — only the public display subset.

type ProfilePublic struct {
	ID          string          `json:"id"`
	DisplayName string          `json:"displayName"`
	AvatarURL   string          `json:"avatarUrl"`
	CoverURL    string          `json:"coverUrl"`
	Bio         string          `json:"bio"`
	SocialLinks []SocialLinkDTO `json:"socialLinks"`
}

type GetProfileReq struct {
	g.Meta `path:"/api/v1/profiles/{id}" method:"get" tags:"profiles" summary:"Public profile by id"`
	ID     string `json:"id" in:"path"`
}

type GetProfileRes struct {
	Profile ProfilePublic `json:"profile"`
}

type BatchProfilesReq struct {
	g.Meta `path:"/api/v1/profiles" method:"get" tags:"profiles" summary:"Public profiles by ids (csv)"`
	IDs    string `json:"ids" in:"query"`
}

type BatchProfilesRes struct {
	Profiles []ProfilePublic `json:"profiles"`
}
