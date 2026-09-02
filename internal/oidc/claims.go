package oidc

import (
	"strings"
	"time"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/model"
)

// scopeSet builds a membership set for quick scope checks.
func scopeSet(scopes []string) map[string]bool {
	m := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		m[s] = true
	}
	return m
}

// BuildSession assembles the fosite session (ID-token + access-token claims) for
// an authenticated identity and the granted scopes. sessionID is the IdP login
// session id (id_session cookie) that binds the minted tokens to the login
// session for passive logout. roles are the identity's granted role slugs; they
// surface in BOTH the ID token and access-token JWT only when the "roles" scope
// was granted. now is injected for tests.
func BuildSession(
	issuer, publicMediaBaseURL, clientID, kid, sessionID, subject string,
	id model.Identity,
	p model.Profile,
	scopes []string,
	roles []string,
	auth authentication.Context,
	issuedAt time.Time,
) *Session {
	has := scopeSet(scopes)
	idExtra := map[string]interface{}{}
	methods := authentication.MethodStrings(auth.Methods)
	accessExtra := map[string]interface{}{
		"client_id":    clientID,
		"subject_kind": "user",
		"auth_time":    auth.AuthenticatedAt.Unix(),
		"amr":          methods,
		"acr":          string(auth.Profile),
		"aal":          string(auth.Level),
	}
	if id.UserKey != "" {
		idExtra["user_key"] = id.UserKey
		accessExtra["user_key"] = id.UserKey
	}
	if has["roles"] {
		if roles == nil {
			roles = []string{}
		}
		idExtra["roles"] = roles
		accessExtra["roles"] = roles
	}
	if has["profile"] {
		idExtra["name"] = p.DisplayName
		idExtra["preferred_username"] = p.Handle
		if picture := profilePictureURL(publicMediaBaseURL, p.AvatarMediaKey); picture != "" {
			idExtra["picture"] = picture
		}
		idExtra["locale"] = p.Locale
	}
	if has["email"] {
		idExtra["email"] = id.Email
		idExtra["email_verified"] = id.EmailVerified
	}
	s := NewAuthenticatedSession(issuer, subject, clientID, kid, idExtra, accessExtra, auth, issuedAt)
	s.IdPSessionID = sessionID
	return s
}

// Userinfo returns the /userinfo response body for the granted scopes. roles are
// the identity's granted role slugs, emitted only when the "roles" scope was
// granted.
func Userinfo(publicMediaBaseURL, subject string, id model.Identity, p model.Profile, scopes []string, roles []string) map[string]interface{} {
	has := scopeSet(scopes)
	out := map[string]interface{}{"sub": subject}
	if id.UserKey != "" {
		out["user_key"] = id.UserKey
	}
	if has["profile"] {
		out["name"] = p.DisplayName
		out["preferred_username"] = p.Handle
		if picture := profilePictureURL(publicMediaBaseURL, p.AvatarMediaKey); picture != "" {
			out["picture"] = picture
		}
		out["locale"] = p.Locale
	}
	if has["email"] {
		out["email"] = id.Email
		out["email_verified"] = id.EmailVerified
	}
	if has["roles"] {
		if roles == nil {
			roles = []string{}
		}
		out["roles"] = roles
	}
	return out
}

func profilePictureURL(publicMediaBaseURL, mediaKey string) string {
	if mediaKey == "" {
		return ""
	}
	return strings.TrimRight(publicMediaBaseURL, "/") + "/" + mediaKey + "?format=webp&name=thumbnail&v=1"
}
