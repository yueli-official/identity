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
	CodeStepUpRequired     = errs.Register("identity.step_up_required", http.StatusPreconditionRequired)
	CodeChallengeRequired  = errs.Register("identity.challenge_required", http.StatusForbidden)
	CodeAbuseUnavailable   = errs.Register("identity.abuse_unavailable", http.StatusServiceUnavailable)
	CodeAbuseReplay        = errs.Register("identity.abuse_attempt_replayed", http.StatusConflict)

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
	CodePasswordAlreadySet = errs.Register("identity.password_already_set", http.StatusConflict)

	CodeIdentityNotFound         = errs.Register("identity.not_found", http.StatusNotFound)
	CodeInvalidStatus            = errs.Register("identity.invalid_status", http.StatusBadRequest)
	CodeCapabilityNotFound       = errs.Register("identity.capability_not_found", http.StatusNotFound)
	CodeProviderNotFound         = errs.Register("identity.provider_not_found", http.StatusNotFound)
	CodeCapabilityProbeRateLimit = errs.Register("identity.capability_probe_rate_limited", http.StatusTooManyRequests)
	CodeCapabilityAudit          = errs.Register("identity.capability_audit_unavailable", http.StatusInternalServerError)
	CodeInvalidGuestRequest      = errs.Register("identity.guest_request_invalid", http.StatusBadRequest)
	CodeInvalidGuestSession      = errs.Register("identity.guest_session_invalid", http.StatusUnauthorized)
	CodeInvalidGuestAudience     = errs.Register("identity.guest_audience_invalid", http.StatusForbidden)
	CodeGuestClaimConflict       = errs.Register("identity.guest_claim_conflict", http.StatusConflict)
	CodePasskeyUnavailable       = errs.Register("identity.passkey_unavailable", http.StatusServiceUnavailable)
	CodePasskeyCeremonyInvalid   = errs.Register("identity.passkey_ceremony_invalid", http.StatusBadRequest)
	CodePasskeyExists            = errs.Register("identity.passkey_exists", http.StatusConflict)
	CodeMFAUnavailable           = errs.Register("identity.mfa_unavailable", http.StatusServiceUnavailable)
	CodeTOTPEnrollmentInvalid    = errs.Register("identity.totp_enrollment_invalid", http.StatusBadRequest)
	CodeTOTPCodeInvalid          = errs.Register("identity.totp_code_invalid", http.StatusUnauthorized)
	CodeTOTPNotFound             = errs.Register("identity.totp_not_found", http.StatusNotFound)
	CodeMFATransactionInvalid    = errs.Register("identity.mfa_transaction_invalid", http.StatusBadRequest)
	CodeRecoveryCodeInvalid      = errs.Register("identity.recovery_code_invalid", http.StatusUnauthorized)
	CodeStepUpRequestInvalid     = errs.Register("identity.step_up_request_invalid", http.StatusBadRequest)
	CodeStepUpMethodUnavailable  = errs.Register("identity.step_up_method_unavailable", http.StatusConflict)
	CodeStepUpProofInvalid       = errs.Register("identity.step_up_proof_invalid", http.StatusUnauthorized)
	CodeStepUpProofReplayed      = errs.Register("identity.step_up_proof_replayed", http.StatusConflict)
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

func ChallengeRequired(attemptID string) *errs.Coded {
	return errs.New(CodeChallengeRequired, "additional verification required", map[string]any{
		"attemptId": attemptID,
		"challenge": "turnstile",
	})
}

func AbuseUnavailable() *errs.Coded {
	return errs.New(CodeAbuseUnavailable, "request admission is temporarily unavailable", nil)
}

func AbuseAttemptReplayed() *errs.Coded {
	return errs.New(CodeAbuseReplay, "request attempt was already admitted", nil)
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

func StepUpRequired(missing []string) *errs.Coded {
	return errs.New(CodeStepUpRequired, "additional authentication required", map[string]any{
		"missing": missing,
	})
}

func CapabilityNotFound(key string) *errs.Coded {
	return errs.New(CodeCapabilityNotFound, "identity capability not found", map[string]any{"key": key})
}

func ProviderNotFound(key string) *errs.Coded {
	return errs.New(CodeProviderNotFound, "identity provider not found", map[string]any{"key": key})
}

func CapabilityProbeRateLimited(key string) *errs.Coded {
	return errs.New(CodeCapabilityProbeRateLimit, "identity provider health check rate limited", map[string]any{"key": key})
}

func CapabilityAuditUnavailable() *errs.Coded {
	return errs.New(CodeCapabilityAudit, "identity capability audit is unavailable", nil)
}

func InvalidGuestRequest() *errs.Coded {
	return errs.New(CodeInvalidGuestRequest, "invalid guest session request", nil)
}

func InvalidGuestSession() *errs.Coded {
	return errs.New(CodeInvalidGuestSession, "guest session is invalid or expired", nil)
}

func InvalidGuestAudience() *errs.Coded {
	return errs.New(CodeInvalidGuestAudience, "guest token audience is not allowed", nil)
}

func GuestClaimConflict() *errs.Coded {
	return errs.New(CodeGuestClaimConflict, "guest session is already claimed", nil)
}

func PasskeyUnavailable() *errs.Coded {
	return errs.New(CodePasskeyUnavailable, "passkey authentication is unavailable", nil)
}

func PasskeyCeremonyInvalid() *errs.Coded {
	return errs.New(CodePasskeyCeremonyInvalid, "passkey ceremony is invalid, expired, or used", nil)
}

func PasskeyExists() *errs.Coded {
	return errs.New(CodePasskeyExists, "passkey is already registered", nil)
}

func MFAUnavailable() *errs.Coded {
	return errs.New(CodeMFAUnavailable, "multi-factor authentication is unavailable", nil)
}

func TOTPEnrollmentInvalid() *errs.Coded {
	return errs.New(CodeTOTPEnrollmentInvalid, "TOTP enrollment is invalid or expired", nil)
}

func TOTPCodeInvalid() *errs.Coded {
	return errs.New(CodeTOTPCodeInvalid, "TOTP code is invalid or already used", nil)
}

func TOTPNotFound() *errs.Coded {
	return errs.New(CodeTOTPNotFound, "TOTP authenticator not found", nil)
}

func MFATransactionInvalid() *errs.Coded {
	return errs.New(CodeMFATransactionInvalid, "MFA transaction is invalid or expired", nil)
}

func RecoveryCodeInvalid() *errs.Coded {
	return errs.New(CodeRecoveryCodeInvalid, "recovery code is invalid or already used", nil)
}

func StepUpRequestInvalid() *errs.Coded {
	return errs.New(CodeStepUpRequestInvalid, "step-up request is invalid", nil)
}

func StepUpMethodUnavailable() *errs.Coded {
	return errs.New(CodeStepUpMethodUnavailable, "no enrolled method can satisfy this step-up requirement", nil)
}

func StepUpProofInvalid() *errs.Coded {
	return errs.New(CodeStepUpProofInvalid, "step-up proof is invalid or does not match this action", nil)
}

func StepUpProofReplayed() *errs.Coded {
	return errs.New(CodeStepUpProofReplayed, "step-up proof was already used", nil)
}

// OAuthEmailConflict: the provider's (unverified) email collides with an
// existing local account, so we refuse to auto-link.
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

// IdentityNotFound: the target identity does not exist (admin operations on a
// user id that matches no row). 404.
func IdentityNotFound() *errs.Coded {
	return errs.New(CodeIdentityNotFound, "user not found", nil)
}

// InvalidStatus: an admin status change requested a value outside the lifecycle
// set {active, disabled, deleted}. 400.
func InvalidStatus(status string) *errs.Coded {
	return errs.New(CodeInvalidStatus, "invalid status", map[string]any{"status": status})
}

// SelfAdminTarget: an admin attempted a destructive action (ban / delete /
// demote) against their own account. Refused to prevent self-lockout. 403.
func SelfAdminTarget() *errs.Coded {
	return errs.New(CodeForbidden, "cannot perform this action on your own account", nil)
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

// LastCredential: refusing to remove the only remaining login credential.
func LastCredential() *errs.Coded {
	return errs.New(CodeLastCredential, "cannot remove the last login credential", nil)
}

// PasswordAlreadySet: SetPassword is for accounts WITHOUT a password (e.g.
// OAuth-only). An account that already has one must use ChangePassword.
func PasswordAlreadySet() *errs.Coded {
	return errs.New(CodePasswordAlreadySet, "password already set; use change password", nil)
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
