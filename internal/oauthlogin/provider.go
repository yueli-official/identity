// Package oauthlogin abstracts external OAuth login providers (e.g. Google) and
// the CSRF-safe signed state passed through the redirect dance. The Provider
// shape is harvested from plugins/auth-oauth (read-only donor).
package oauthlogin

import "context"

// UserInfo is the normalized profile an OAuth provider returns.
type UserInfo struct {
	ProviderUID   string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

// Provider abstracts an external OAuth login source.
type Provider interface {
	Name() string
	AuthorizeURL(state string) string
	ExchangeCode(ctx context.Context, code string) (accessToken string, err error)
	FetchUserInfo(ctx context.Context, accessToken string) (UserInfo, error)
}
