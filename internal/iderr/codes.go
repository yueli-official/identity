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
