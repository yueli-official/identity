package v1

import "github.com/gogf/gf/v2/frame/g"

type StepUpRequirement struct {
	FreshWithinSeconds int    `json:"freshWithinSeconds,omitempty" v:"between:0,3600"`
	MinimumLevel       string `json:"minimumLevel,omitempty"`
	MinimumProfile     string `json:"minimumProfile,omitempty"`
	UserVerification   bool   `json:"userVerification,omitempty"`
	PhishingResistant  bool   `json:"phishingResistant,omitempty"`
	MinimumFactorCount int    `json:"minimumFactorCount,omitempty" v:"between:0,3"`
}

type StepUpBeginReq struct {
	g.Meta      `path:"/api/v1/auth/step-up/begin" method:"post" tags:"step-up" summary:"Begin an action-bound step-up"`
	Audience    string            `json:"audience" v:"required|length:1,200"`
	Action      string            `json:"action" v:"required|length:1,200"`
	Resource    string            `json:"resource" v:"required|length:1,2048"`
	Requirement StepUpRequirement `json:"requirement"`
}

type StepUpBeginRes struct {
	Satisfied     bool     `json:"satisfied"`
	Proof         string   `json:"proof,omitempty"`
	TransactionID string   `json:"transactionId,omitempty"`
	ExpiresAt     string   `json:"expiresAt,omitempty"`
	Methods       []string `json:"methods,omitempty"`
}

type StepUpTOTPFinishReq struct {
	g.Meta        `path:"/api/v1/auth/step-up/totp" method:"post" tags:"step-up" summary:"Finish an action-bound TOTP step-up"`
	TransactionID string `json:"transactionId" v:"required"`
	Code          string `json:"code" v:"required|regex:^\\d{6}$"`
}

type StepUpTOTPFinishRes struct {
	Proof string `json:"proof"`
}
