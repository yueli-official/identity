package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/yueli-official/foundation/go/privacy"
)

// OpenErasureReq uses caller-generated stable values so HTTP retries replay the
// same immutable command instead of silently opening a second request.
type OpenErasureReq struct {
	g.Meta `path:"/api/v1/session/privacy/erasure" method:"post" tags:"privacy" summary:"Open an account erasure request"`

	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:200"`
	StatusToken    string `json:"statusToken" v:"required|min-length:32|max-length:200"`
	RequestedAt    string `json:"requestedAt" v:"required"`
}

type OpenErasureRes struct {
	Request privacy.RightsRequestView `json:"request"`
}

// PrivacyRequestStatusReq is capability-authenticated with the secret returned
// only to the caller. It continues to work after Identity is the final Owner
// and the login account has been erased.
type PrivacyRequestStatusReq struct {
	g.Meta `path:"/api/v1/privacy/requests/{id}/status" method:"post" tags:"privacy" summary:"Get and advance a privacy request"`

	ID          string `json:"id" in:"path" v:"required"`
	StatusToken string `json:"statusToken" v:"required|min-length:32|max-length:200"`
}

type PrivacyRequestStatusRes struct {
	Request privacy.RightsRequestView `json:"request"`
}
