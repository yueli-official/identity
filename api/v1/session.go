package v1

import "github.com/gogf/gf/v2/frame/g"

type MeReq struct {
	g.Meta `path:"/api/v1/session/me" method:"get" tags:"session" summary:"Current identity"`
}
type MeRes struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"emailVerified"`
}
