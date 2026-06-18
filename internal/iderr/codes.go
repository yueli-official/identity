// Package iderr registers identity-service error codes into the gokit registry
// and provides typed constructors. (TS catalog/codegen lands with Plan 2-UI;
// the Go registry is the single source of truth for now.)
package iderr

import (
	"net/http"

	"platform/gokit/errs"
)

var (
	CodeEmailTaken         = errs.Register("identity.email_taken", http.StatusConflict)
	CodeInvalidCredentials = errs.Register("identity.invalid_credentials", http.StatusUnauthorized)
	CodeAccountDisabled    = errs.Register("identity.account_disabled", http.StatusForbidden)
	CodeAccountLocked      = errs.Register("identity.account_locked", http.StatusTooManyRequests)
	CodeWeakPassword       = errs.Register("identity.weak_password", http.StatusBadRequest)
	CodeInvalidEmail       = errs.Register("identity.invalid_email", http.StatusBadRequest)
	CodeNotAuthenticated   = errs.Register("identity.not_authenticated", http.StatusUnauthorized)

	CodeOAuthEmailConflict = errs.Register("identity.oauth_email_conflict", http.StatusConflict)
	CodeOAuthNoEmail       = errs.Register("identity.oauth_no_email", http.StatusBadRequest)
	CodeOAuthFailed        = errs.Register("identity.oauth_failed", http.StatusUnauthorized)

	CodeVerificationInvalid = errs.Register("identity.verification_invalid", http.StatusBadRequest)
	CodeResetThrottled      = errs.Register("identity.reset_throttled", http.StatusTooManyRequests)
	CodeVerifyThrottled     = errs.Register("identity.verify_throttled", http.StatusTooManyRequests)

	CodeForbidden   = errs.Register("identity.forbidden", http.StatusForbidden)
	CodeUnknownRole = errs.Register("identity.unknown_role", http.StatusBadRequest)

	CodeInvalidProfile  = errs.Register("identity.invalid_profile", http.StatusBadRequest)
	CodeSessionNotFound = errs.Register("identity.session_not_found", http.StatusNotFound)

	CodeCredentialConflict = errs.Register("identity.credential_conflict", http.StatusConflict)
	CodeCredentialNotFound = errs.Register("identity.credential_not_found", http.StatusNotFound)
	CodeLastCredential     = errs.Register("identity.last_credential", http.StatusConflict)
)

func EmailTaken(email string) *errs.Coded {
	return errs.New(CodeEmailTaken, "email already registered", map[string]any{"email": email})
}

// InvalidCredentials is intentionally generic (no enumeration of which field).
func InvalidCredentials() *errs.Coded {
	return errs.New(CodeInvalidCredentials, "invalid email or password", nil)
}

func AccountDisabled() *errs.Coded {
	return errs.New(CodeAccountDisabled, "account disabled", nil)
}

func AccountLocked() *errs.Coded {
	return errs.New(CodeAccountLocked, "too many attempts, try again later", nil)
}

func WeakPassword(reason string) *errs.Coded {
	return errs.New(CodeWeakPassword, "password too weak", map[string]any{"reason": reason})
}

func InvalidEmail(email string) *errs.Coded {
	return errs.New(CodeInvalidEmail, "invalid email format", map[string]any{"email": email})
}

func NotAuthenticated() *errs.Coded {
	return errs.New(CodeNotAuthenticated, "not authenticated", nil)
}

// OAuthEmailConflict: the provider's (unverified) email collides with an
// existing local account, so we refuse to auto-link (spec §10).
func OAuthEmailConflict(email string) *errs.Coded {
	return errs.New(CodeOAuthEmailConflict, "email already registered to another account", map[string]any{"email": email})
}

// OAuthNoEmail: the provider returned no email, so we can neither link nor register.
func OAuthNoEmail() *errs.Coded {
	return errs.New(CodeOAuthNoEmail, "oauth provider returned no email", nil)
}

// OAuthFailed is a generic provider/exchange failure for any non-redirect caller
// that needs a coded error (the redirect endpoints surface errors via query string).
func OAuthFailed() *errs.Coded {
	return errs.New(CodeOAuthFailed, "oauth login failed", nil)
}

// VerificationInvalid: a verify/reset token is missing, expired, already used, or
// scoped to a different purpose (intentionally generic — no detail leak).
func VerificationInvalid() *errs.Coded {
	return errs.New(CodeVerificationInvalid, "verification token invalid, expired, or used", nil)
}

// ResetThrottled: too many password-reset requests for this account/IP.
func ResetThrottled() *errs.Coded {
	return errs.New(CodeResetThrottled, "too many reset requests, try again later", nil)
}

// VerifyThrottled: too many email-verification requests for this account/IP.
func VerifyThrottled() *errs.Coded {
	return errs.New(CodeVerifyThrottled, "too many verification requests, try again later", nil)
}

// Forbidden: the caller is authenticated but lacks the required privilege
// (e.g. a non-admin hitting an admin-only endpoint).
func Forbidden() *errs.Coded {
	return errs.New(CodeForbidden, "insufficient privileges", nil)
}

// UnknownRole: the requested role slug is not in the fixed catalog (migration
// 0006 seeds {user, admin}). A client error, not a server fault — so it maps to
// 400 rather than leaking through the envelope as a generic 500.
func UnknownRole(slug string) *errs.Coded {
	return errs.New(CodeUnknownRole, "unknown role slug", map[string]any{"role": slug})
}

// InvalidProfile: a submitted profile field failed validation (e.g. empty
// display name).
func InvalidProfile(reason string) *errs.Coded {
	return errs.New(CodeInvalidProfile, "invalid profile", map[string]any{"reason": reason})
}

// SessionNotFound: the target session does not exist OR is not owned by the
// caller (intentionally merged — don't reveal another account's sessions).
func SessionNotFound() *errs.Coded {
	return errs.New(CodeSessionNotFound, "session not found", nil)
}

// CredentialConflict: the external account is already linked to a different identity.
func CredentialConflict(provider string) *errs.Coded {
	return errs.New(CodeCredentialConflict, "this account is already linked to another identity", map[string]any{"provider": provider})
}

// CredentialNotFound: the caller has no such credential to unbind.
func CredentialNotFound() *errs.Coded {
	return errs.New(CodeCredentialNotFound, "credential not found", nil)
}

// LastCredential: refusing to remove the only remaining login credential (spec §10).
func LastCredential() *errs.Coded {
	return errs.New(CodeLastCredential, "cannot remove the last login credential", nil)
}

// ── Personal Access Token (PAT) codes ───────────────────────────────────────

var (
	CodePATNameRequired   = errs.Register("identity.pat_name_required", http.StatusBadRequest)
	CodePATScopesRequired = errs.Register("identity.pat_scopes_required", http.StatusBadRequest)
	CodePATScopeInvalid   = errs.Register("identity.pat_scope_invalid", http.StatusBadRequest)
	CodePATLimitReached   = errs.Register("identity.pat_limit_reached", http.StatusConflict)
	CodePATNotFound       = errs.Register("identity.pat_not_found", http.StatusNotFound)
	CodePATInvalid        = errs.Register("identity.pat_invalid", http.StatusUnauthorized)
	CodePATExpired        = errs.Register("identity.pat_expired", http.StatusUnauthorized)
)

func PATNameRequired() *errs.Coded {
	return errs.New(CodePATNameRequired, "token name required", nil)
}

func PATScopesRequired() *errs.Coded {
	return errs.New(CodePATScopesRequired, "at least one scope required", nil)
}

func PATScopeInvalid() *errs.Coded {
	return errs.New(CodePATScopeInvalid, "scope is invalid", nil)
}

func PATLimitReached(max int) *errs.Coded {
	return errs.New(CodePATLimitReached, "personal access token limit reached", map[string]any{"max": max})
}

func PATNotFound() *errs.Coded {
	return errs.New(CodePATNotFound, "token not found", nil)
}

// PATInvalid is intentionally generic (no enumeration of why the token is bad).
func PATInvalid() *errs.Coded {
	return errs.New(CodePATInvalid, "invalid token", nil)
}

func PATExpired() *errs.Coded {
	return errs.New(CodePATExpired, "token has expired", nil)
}
