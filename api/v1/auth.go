// Package v1 holds identity-service request/response types. g.Meta drives the
// auto-generated OpenAPI document.
package v1

import "github.com/gogf/gf/v2/frame/g"

type RegisterReq struct {
	g.Meta         `path:"/api/v1/auth/register" method:"post" tags:"auth" summary:"Register with email+password"`
	Email          string `json:"email" v:"required|email"`
	Password       string `json:"password" v:"required"`
	DisplayName    string `json:"displayName"`
	AbuseAttemptID string `json:"abuseAttemptId,omitempty"`
	ChallengeProof string `json:"challengeProof,omitempty"`
}
type RegisterRes struct {
	UserKey string `json:"userKey"`
	Email   string `json:"email"`
}

type LoginReq struct {
	g.Meta         `path:"/api/v1/auth/login" method:"post" tags:"auth" summary:"Email+password login"`
	Email          string `json:"email" v:"required|email"`
	Password       string `json:"password" v:"required"`
	AbuseAttemptID string `json:"abuseAttemptId,omitempty"`
	ChallengeProof string `json:"challengeProof,omitempty"`
}
type LoginRes struct {
	UserKey        string   `json:"userKey"`
	Email          string   `json:"email"`
	MFARequired    bool     `json:"mfaRequired"`
	MFATransaction string   `json:"mfaTransaction,omitempty"`
	MFAExpiresAt   string   `json:"mfaExpiresAt,omitempty"`
	MFAMethods     []string `json:"mfaMethods,omitempty"`
}

type LogoutReq struct {
	g.Meta `path:"/api/v1/auth/logout" method:"post" tags:"auth" summary:"Log out current session"`
}
type LogoutRes struct{}

// EmailVerifyRequestReq sends a verify-email link to the logged-in user. The
// caller is resolved from the session cookie; no body is needed.
type EmailVerifyRequestReq struct {
	g.Meta `path:"/api/v1/auth/email/verify-request" method:"post" tags:"auth" summary:"Send an email-verification link to the logged-in user"`
}
type EmailVerifyRequestRes struct{}

// EmailVerifyReq consumes an email-verification token from the emailed link.
type EmailVerifyReq struct {
	g.Meta `path:"/api/v1/auth/email/verify" method:"post" tags:"auth" summary:"Consume an email-verification token"`
	Token  string `json:"token" v:"required"`
}
type EmailVerifyRes struct{}

// PasswordForgotReq requests a password-reset email. Always returns success to
// avoid account enumeration.
type PasswordForgotReq struct {
	g.Meta `path:"/api/v1/auth/password/forgot" method:"post" tags:"auth" summary:"Request a password-reset email"`
	Email  string `json:"email" v:"required|email"`
}
type PasswordForgotRes struct{}

// PasswordResetReq sets a new password via a single-use reset token.
type PasswordResetReq struct {
	g.Meta   `path:"/api/v1/auth/password/reset" method:"post" tags:"auth" summary:"Reset password via token"`
	Token    string `json:"token" v:"required"`
	Password string `json:"password" v:"required"`
}
type PasswordResetRes struct{}

type PasswordPolicyReq struct {
	g.Meta `path:"/api/v1/auth/password-policy" method:"get" tags:"auth" summary:"Get the active password creation policy"`
}

type PasswordPolicyRes struct {
	MinLength     int    `json:"minLength"`
	MaxLength     int    `json:"maxLength"`
	Normalization string `json:"normalization"`
	Blocklist     bool   `json:"blocklist"`
}
