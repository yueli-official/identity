package v1

import "github.com/gogf/gf/v2/frame/g"

// PublicUser is the stable public projection. userKey is the immutable compact
// public locator and handle is a mutable alias.
type PublicUser struct {
	UserKey     string          `json:"userKey"`
	Handle      string          `json:"handle"`
	DisplayName string          `json:"displayName"`
	Avatar      *MediaRef       `json:"avatar,omitempty"`
	Cover       *MediaRef       `json:"cover,omitempty"`
	Bio         string          `json:"bio"`
	SocialLinks []SocialLinkDTO `json:"socialLinks"`
}

type MediaRef struct {
	MediaKey string `json:"mediaKey" v:"required|regex:^[0-9A-Za-z]{20,32}$"`
}

type GetUserReq struct {
	g.Meta  `path:"/api/v1/users/{userKey}" method:"get" tags:"users" summary:"Public user by stable key"`
	UserKey string `json:"userKey" in:"path" v:"required|regex:^[1-9A-HJ-NP-Za-km-z]{8}$"`
}

type GetUserRes struct {
	User PublicUser `json:"user"`
}

type GetUserByHandleReq struct {
	g.Meta `path:"/api/v1/users/by-handle/{handle}" method:"get" tags:"users" summary:"Public user by current or historical handle"`
	Handle string `json:"handle" in:"path" v:"required|length:3,30|regex:^[A-Za-z0-9][A-Za-z0-9_]{1,28}[A-Za-z0-9]$"`
}

type GetUserByHandleRes struct {
	User PublicUser `json:"user"`
}

type BatchUsersReq struct {
	g.Meta `path:"/api/v1/users" method:"get" tags:"users" summary:"Public users by stable keys (csv)"`
	IDs    string `json:"ids" in:"query" v:"required|max-length:2699"`
}

type BatchUsersRes struct {
	Users []PublicUser `json:"users"`
}
