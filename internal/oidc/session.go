package oidc

import (
	"time"

	"github.com/mohae/deepcopy"
	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/token/jwt"
)

// Session is the fosite session type used throughout the identity service.
// It satisfies BOTH:
//   - openid.Session  (via the embedded *openid.DefaultSession), and
//   - oauth2.JWTSessionContainer (via GetJWTClaims/GetJWTHeader below).
//
// openid.DefaultSession alone is NOT a JWTSessionContainer — it exposes
// IDTokenClaims()/IDTokenHeaders(), not GetJWTClaims()/GetJWTHeader(). We
// bridge the gap here, which is the mandatory cost of JWT access tokens with
// fosite (proven by the PoC).
type Session struct {
	*openid.DefaultSession
	// JWTClaims and JWTHeader feed the ACCESS token JWT (separate from the ID
	// token, whose claims live in DefaultSession.Claims).
	JWTClaims *jwt.JWTClaims
	JWTHeader *jwt.Headers
}

// GetJWTClaims implements oauth2.JWTSessionContainer.
func (s *Session) GetJWTClaims() jwt.JWTClaimsContainer {
	if s.JWTClaims == nil {
		s.JWTClaims = &jwt.JWTClaims{}
	}
	return s.JWTClaims
}

// GetJWTHeader implements oauth2.JWTSessionContainer.
func (s *Session) GetJWTHeader() *jwt.Headers {
	if s.JWTHeader == nil {
		s.JWTHeader = &jwt.Headers{}
	}
	return s.JWTHeader
}

// Clone implements fosite.Session. fosite stores and re-hydrates sessions, so
// the copy must be deep (no shared pointer state between the stored original
// and the live request).
func (s *Session) Clone() fosite.Session {
	if s == nil {
		return nil
	}
	return deepcopy.Copy(s).(fosite.Session)
}

// NewSession builds a session for an authenticated subject. idExtra feeds the
// ID-token's extra claims; accessExtra feeds the access-token JWT's extra
// claims. now is injected for testability.
func NewSession(issuer, subject, clientID, kid string, idExtra, accessExtra map[string]interface{}, now time.Time) *Session {
	return &Session{
		DefaultSession: &openid.DefaultSession{
			Claims: &jwt.IDTokenClaims{
				Issuer:      issuer,
				Subject:     subject,
				Audience:    []string{clientID},
				IssuedAt:    now,
				RequestedAt: now,
				AuthTime:    now,
				Extra:       idExtra,
			},
			Headers: &jwt.Headers{Extra: map[string]interface{}{"kid": kid}},
			Subject: subject,
		},
		JWTClaims: &jwt.JWTClaims{Subject: subject, Extra: accessExtra},
		JWTHeader: &jwt.Headers{Extra: map[string]interface{}{"kid": kid}},
	}
}

// EmptySession returns a hydratable empty session for the token/introspection
// endpoints (fosite fills it from stored state). All inner pointers are
// non-nil so fosite can populate them without a nil-dereference.
func EmptySession() *Session {
	return &Session{
		DefaultSession: &openid.DefaultSession{
			Claims:  &jwt.IDTokenClaims{},
			Headers: &jwt.Headers{},
		},
		JWTClaims: &jwt.JWTClaims{},
		JWTHeader: &jwt.Headers{},
	}
}
