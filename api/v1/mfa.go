package v1

import "github.com/gogf/gf/v2/frame/g"

type TOTPLoginReq struct {
	g.Meta        `path:"/api/v1/auth/mfa/totp" method:"post" tags:"mfa" summary:"Complete login with a TOTP code"`
	TransactionID string `json:"transactionId" v:"required"`
	Code          string `json:"code" v:"required|regex:^\\d{6}$"`
}

type TOTPLoginRes struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type RecoveryLoginReq struct {
	g.Meta        `path:"/api/v1/auth/mfa/recovery" method:"post" tags:"mfa" summary:"Use a recovery code for restricted account recovery"`
	TransactionID string `json:"transactionId" v:"required"`
	Code          string `json:"code" v:"required|length:16,32"`
}

type RecoveryLoginRes struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Restricted bool   `json:"restricted"`
}

type RecoverySessionReq struct {
	g.Meta `path:"/api/v1/account/recovery" method:"get" tags:"mfa" summary:"Inspect a restricted recovery session"`
}

type RecoverySessionRes struct {
	IdentityID string `json:"identityId"`
	ExpiresAt  string `json:"expiresAt"`
}

type TOTPEnrollmentBeginReq struct {
	g.Meta `path:"/api/v1/account/mfa/totp/enrollment/begin" method:"post" tags:"mfa" summary:"Begin TOTP enrollment"`
	Label  string `json:"label" v:"length:0,100"`
}

type TOTPEnrollmentBeginRes struct {
	AuthenticatorID string `json:"authenticatorId"`
	URI             string `json:"uri"`
	Secret          string `json:"secret"`
	ExpiresAt       string `json:"expiresAt"`
}

type TOTPEnrollmentFinishReq struct {
	g.Meta          `path:"/api/v1/account/mfa/totp/enrollment/finish" method:"post" tags:"mfa" summary:"Finish TOTP enrollment"`
	AuthenticatorID string `json:"authenticatorId" v:"required"`
	Code            string `json:"code" v:"required|regex:^\\d{6}$"`
}

type TOTPEnrollmentFinishRes struct {
	Authenticator TOTPEntry `json:"authenticator"`
	RecoveryCodes []string  `json:"recoveryCodes"`
}

type ListTOTPReq struct {
	g.Meta `path:"/api/v1/account/mfa/totp" method:"get" tags:"mfa" summary:"List TOTP authenticators"`
}

type ListTOTPRes struct {
	Entries []TOTPEntry `json:"entries"`
}

type RevokeTOTPReq struct {
	g.Meta `path:"/api/v1/account/mfa/totp/{id}" method:"delete" tags:"mfa" summary:"Remove a TOTP authenticator"`
	ID     string `json:"id" in:"path" v:"required"`
}

type RevokeTOTPRes struct{}

type TOTPEntry struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	VerifiedAt string `json:"verifiedAt,omitempty"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}
