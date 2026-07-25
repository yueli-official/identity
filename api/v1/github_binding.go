package v1

import "github.com/gogf/gf/v2/frame/g"

type BeginGitHubBindingReq struct {
	g.Meta   `path:"/api/v1/account/github-bindings/authorization" method:"post" tags:"publisher,github" summary:"Begin GitHub account ownership verification" security:"UserAuth"`
	ReturnTo string `json:"returnTo" v:"max-length:1024"`
}

type BeginGitHubBindingRes struct {
	AuthorizationURL string `json:"authorizationUrl"`
	ExpiresAt        string `json:"expiresAt"`
}

type ListGitHubBindingsReq struct {
	g.Meta `path:"/api/v1/account/github-bindings" method:"get" tags:"publisher,github" summary:"List GitHub binding history" security:"UserAuth"`
}

type GitHubBindingDTO struct {
	ID                string `json:"id"`
	ProviderAccountID string `json:"providerAccountId"`
	ProviderNodeID    string `json:"providerNodeId,omitempty"`
	Login             string `json:"login"`
	AvatarURL         string `json:"avatarUrl,omitempty"`
	Status            string `json:"status"`
	VerifiedAt        string `json:"verifiedAt"`
	LastVerifiedAt    string `json:"lastVerifiedAt"`
	UnboundAt         string `json:"unboundAt,omitempty"`
	BlockedAt         string `json:"blockedAt,omitempty"`
}

type ListGitHubBindingsRes struct {
	Bindings []GitHubBindingDTO `json:"bindings"`
}

type UnbindGitHubBindingReq struct {
	g.Meta    `path:"/api/v1/account/github-bindings/{bindingId}" method:"delete" tags:"publisher,github" summary:"Unbind a GitHub account while retaining history" security:"UserAuth"`
	BindingID string `json:"bindingId" in:"path" v:"required|uuid"`
}

type UnbindGitHubBindingRes struct {
	Binding GitHubBindingDTO `json:"binding"`
}
