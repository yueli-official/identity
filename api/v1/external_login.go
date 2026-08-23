package v1

import "github.com/gogf/gf/v2/frame/g"

type ExternalLoginProviderView struct {
	Key                string `json:"key"`
	Label              string `json:"label"`
	RegistrationPolicy string `json:"registrationPolicy"`
	Configured         bool   `json:"configured"`
	Enabled            bool   `json:"enabled"`
	ClientID           string `json:"clientId,omitempty"`
	RedirectURL        string `json:"redirectUrl"`
	SecretVersion      int    `json:"secretVersion"`
	LastHealthOK       *bool  `json:"lastHealthOk,omitempty"`
	LastHealthChecked  string `json:"lastHealthCheckedAt,omitempty"`
	LastHealthError    string `json:"lastHealthError,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

type PublicExternalLoginProvidersReq struct {
	g.Meta `path:"/api/v1/auth/oauth/providers" method:"get" tags:"auth" summary:"List enabled external login providers"`
}

type PublicExternalLoginProvider struct {
	Key                string `json:"key"`
	Label              string `json:"label"`
	RegistrationPolicy string `json:"registrationPolicy"`
}

type PublicExternalLoginProvidersRes struct {
	Entries []PublicExternalLoginProvider `json:"entries"`
}

type AdminExternalLoginProvidersReq struct {
	g.Meta `path:"/api/v1/admin/login-providers" method:"get" tags:"admin" summary:"List external login provider configuration"`
}

type AdminExternalLoginProvidersRes struct {
	Entries []ExternalLoginProviderView `json:"entries"`
}

type AdminSaveExternalLoginProviderReq struct {
	g.Meta       `path:"/api/v1/admin/login-providers/{key}" method:"put" tags:"admin" summary:"Configure an external login provider"`
	Key          string `json:"key" in:"path"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Enabled      bool   `json:"enabled"`
}

type AdminSaveExternalLoginProviderRes struct {
	Provider ExternalLoginProviderView `json:"provider"`
}

type AdminCheckExternalLoginProviderReq struct {
	g.Meta `path:"/api/v1/admin/login-providers/{key}/health-check" method:"post" tags:"admin" summary:"Check external login provider connectivity"`
	Key    string `json:"key" in:"path"`
}

type AdminCheckExternalLoginProviderRes struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}
