package v1

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

type GuestSessionCreateReq struct {
	g.Meta              `path:"/api/v1/guest/sessions" method:"post" tags:"guest-session" summary:"Create a durable guest session"`
	ClientID            string `json:"clientId" v:"required"`
	RequestedTTLSeconds int64  `json:"requestedTtlSeconds" v:"required|min:1"`
}

type GuestSessionCreateRes struct {
	SubjectID           string    `json:"subjectId"`
	SessionToken        string    `json:"sessionToken"`
	EffectiveTTLSeconds int64     `json:"effectiveTtlSeconds"`
	ExpiresAt           time.Time `json:"expiresAt"`
}

type GuestTokenReq struct {
	g.Meta       `path:"/api/v1/guest/tokens" method:"post" tags:"guest-session" summary:"Issue a short-lived resource token for a guest session"`
	ClientID     string `json:"clientId" v:"required"`
	SessionToken string `json:"sessionToken" v:"required"`
	Audience     string `json:"audience" v:"required"`
}

type GuestTokenRes struct {
	AccessToken      string `json:"accessToken"`
	ExpiresInSeconds int64  `json:"expiresInSeconds"`
}

type GuestSessionClaimReq struct {
	g.Meta       `path:"/api/v1/guest/sessions/claim" method:"post" tags:"guest-session" summary:"Claim a guest session after sign-in" security:"UserAuth"`
	ClientID     string `json:"clientId" v:"required"`
	SessionToken string `json:"sessionToken" v:"required"`
}

type GuestSessionClaimRes struct {
	SubjectID string    `json:"subjectId"`
	UserID    string    `json:"userId"`
	ClaimedAt time.Time `json:"claimedAt"`
}
