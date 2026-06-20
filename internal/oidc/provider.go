package oidc

import (
	"context"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/token/jwt"
)

// Config carries OIDC provider settings (from service config / env).
type Config struct {
	Issuer       string
	GlobalSecret []byte // >= 32 bytes; backs the HMAC auth-code signatures
	AccessTTL    time.Duration
	IDTTL        time.Duration
	RefreshTTL   time.Duration // refresh token lifespan (e.g. 30d)
}

// NewProvider builds the fosite OAuth2 provider with a JWT access-token
// strategy (RS256 via keyGetter) — NOT ComposeAllEnabled (which issues opaque
// tokens). Factories: authorization_code + OIDC + PKCE + introspection (userinfo
// uses IntrospectToken), plus refresh and revocation.
func NewProvider(store fosite.Storage, cfg Config, keyGetter func(context.Context) (interface{}, error)) fosite.OAuth2Provider {
	fcfg := &fosite.Config{
		AccessTokenLifespan:            cfg.AccessTTL,
		RefreshTokenLifespan:           cfg.RefreshTTL,
		IDTokenLifespan:                cfg.IDTTL,
		IDTokenIssuer:                  cfg.Issuer,
		AccessTokenIssuer:              cfg.Issuer,
		ScopeStrategy:                  fosite.WildcardScopeStrategy,
		JWTScopeClaimKey:               jwt.JWTScopeFieldString,
		EnforcePKCEForPublicClients:    true,
		EnablePKCEPlainChallengeMethod: false,
		GlobalSecret:                   cfg.GlobalSecret,
	}
	strat := &compose.CommonStrategy{
		CoreStrategy: compose.NewOAuth2JWTStrategy(
			keyGetter,
			compose.NewOAuth2HMACStrategy(fcfg),
			fcfg,
		),
		OpenIDConnectTokenStrategy: compose.NewOpenIDConnectStrategy(keyGetter, fcfg),
		Signer:                     &jwt.DefaultSigner{GetPrivateKey: keyGetter},
	}
	return compose.Compose(
		fcfg, store, strat,
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OpenIDConnectExplicitFactory,
		compose.OAuth2PKCEFactory,
		// Stateless JWT introspection: access tokens are self-contained JWTs and
		// are NOT persisted (Store.GetAccessTokenSession no-ops), so the stateful
		// CoreValidator (OAuth2TokenIntrospectionFactory) would fail every lookup.
		// Validate from the JWT itself instead (userinfo path).
		compose.OAuth2StatelessJWTIntrospectionFactory,
		compose.OAuth2RefreshTokenGrantFactory, // refresh_token grant + rotation
		compose.OAuth2TokenRevocationFactory,   // RFC 7009 /revoke
		// Service-to-service tokens: a confidential client (e.g. the resource
		// site) authenticates with its secret and mints a scoped access token for
		// itself (no user). Powers cross-service calls like the asset service's
		// service-scope signed delivery.
		compose.OAuth2ClientCredentialsGrantFactory,
	)
}
