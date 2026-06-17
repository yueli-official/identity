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
}

// NewProvider builds the fosite OAuth2 provider with a JWT access-token
// strategy (RS256 via keyGetter) — NOT ComposeAllEnabled (which issues opaque
// tokens). Milestone-③ factories: authorization_code + OIDC + PKCE +
// introspection (userinfo uses IntrospectToken). Refresh/revocation are
// milestone ④.
func NewProvider(store fosite.Storage, cfg Config, keyGetter func(context.Context) (interface{}, error)) fosite.OAuth2Provider {
	fcfg := &fosite.Config{
		AccessTokenLifespan:            cfg.AccessTTL,
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
	)
}
