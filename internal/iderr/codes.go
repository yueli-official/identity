package iderr

import (
	"time"
)

func EmailTaken(email string) error {
	return mapped(CodeEmailTaken, map[string]any{"email": email})
}

// InvalidCredentials is intentionally generic (no enumeration of which field).
func InvalidCredentials() error {
	return mapped(CodeInvalidCredentials, nil)
}

func AccountDisabled() error {
	return mapped(CodeAccountDisabled, nil)
}

func AccountLocked() error {
	return mapped(CodeAccountLocked, nil)
}

func AccountLockedUntil(retryAt time.Time) error {
	if retryAt.IsZero() {
		return AccountLocked()
	}
	return mapped(CodeAccountLocked, retryAtParams(retryAt))
}

func ChallengeRequired(attemptID string) error {
	return mapped(CodeChallengeRequired, map[string]any{
		"attemptId": attemptID,
		"challenge": "turnstile",
	})
}

func AbuseUnavailable() error {
	return mapped(CodeAbuseUnavailable, nil)
}

func AbuseAttemptReplayed() error {
	return mapped(CodeAbuseReplay, nil)
}

func WeakPassword(reason string) error {
	return mapped(CodeWeakPassword, map[string]any{"reason": reason})
}

func InvalidEmail(email string) error {
	return mapped(CodeInvalidEmail, map[string]any{"email": email})
}

func NotAuthenticated() error {
	return mapped(CodeNotAuthenticated, nil)
}

func StepUpRequired(missing []string) error {
	return mapped(CodeStepUpRequired, map[string]any{
		"missing": missing,
	})
}

func CapabilityNotFound(key string) error {
	return mapped(CodeCapabilityNotFound, map[string]any{"key": key})
}

func ProviderNotFound(key string) error {
	return mapped(CodeProviderNotFound, map[string]any{"key": key})
}

func CapabilityProbeRateLimited(key string) error {
	return mapped(CodeCapabilityProbeRateLimit, map[string]any{"key": key})
}

func CapabilityAuditUnavailable() error {
	return mapped(CodeCapabilityAudit, nil)
}

func InvalidGuestRequest() error {
	return mapped(CodeInvalidGuestRequest, nil)
}

func InvalidGuestSession() error {
	return mapped(CodeInvalidGuestSession, nil)
}

func InvalidGuestAudience() error {
	return mapped(CodeInvalidGuestAudience, nil)
}

func GuestClaimConflict() error {
	return mapped(CodeGuestClaimConflict, nil)
}

func PasskeyUnavailable() error {
	return mapped(CodePasskeyUnavailable, nil)
}

func PasskeyCeremonyInvalid() error {
	return mapped(CodePasskeyCeremonyInvalid, nil)
}

func PasskeyExists() error {
	return mapped(CodePasskeyExists, nil)
}

func MFAUnavailable() error {
	return mapped(CodeMFAUnavailable, nil)
}

func TOTPEnrollmentInvalid() error {
	return mapped(CodeTOTPEnrollmentInvalid, nil)
}

func TOTPCodeInvalid() error {
	return mapped(CodeTOTPCodeInvalid, nil)
}

func TOTPNotFound() error {
	return mapped(CodeTOTPNotFound, nil)
}

func MFATransactionInvalid() error {
	return mapped(CodeMFATransactionInvalid, nil)
}

func RecoveryCodeInvalid() error {
	return mapped(CodeRecoveryCodeInvalid, nil)
}

func StepUpRequestInvalid() error {
	return mapped(CodeStepUpRequestInvalid, nil)
}

func StepUpMethodUnavailable() error {
	return mapped(CodeStepUpMethodUnavailable, nil)
}

func StepUpProofInvalid() error {
	return mapped(CodeStepUpProofInvalid, nil)
}

func StepUpProofReplayed() error {
	return mapped(CodeStepUpProofReplayed, nil)
}

func PublisherConsumerNotFound() error {
	return mapped(CodePublisherConsumerNotFound, nil)
}

func PublisherConsumerDisabled() error {
	return mapped(CodePublisherConsumerDisabled, nil)
}

type PublisherAttestationInvalidReason string

const (
	PublisherAttestationReasonCommand     PublisherAttestationInvalidReason = "command_invalid"
	PublisherAttestationReasonAttestation PublisherAttestationInvalidReason = "attestation_invalid"
)

func PublisherAttestationInvalid(reason PublisherAttestationInvalidReason) error {
	return mapped(CodePublisherAttestationInvalid, map[string]any{
		"reason": string(reason),
	})
}

func PublisherIdempotencyConflict() error {
	return mapped(CodePublisherIdempotencyConflict, nil)
}

func PublisherSigningUnavailable() error {
	return mapped(CodePublisherSigningUnavailable, nil)
}

func PublisherRotationPending() error {
	return mapped(CodePublisherRotationPending, nil)
}

func PublisherKeyTransitionInvalid() error {
	return mapped(CodePublisherKeyTransition, nil)
}

func PublisherTrustManifestInvalid() error {
	return mapped(CodePublisherTrustInvalid, nil)
}

func PublisherRootUntrusted() error {
	return mapped(CodePublisherRootUntrusted, nil)
}

func GitHubBindingUnavailable() error {
	return mapped(CodeGitHubBindingUnavailable, nil)
}

func GitHubBindingAttemptInvalid() error {
	return mapped(CodeGitHubBindingAttemptInvalid, nil)
}

func GitHubBindingConflict() error {
	return mapped(CodeGitHubBindingConflict, nil)
}

func GitHubBindingNotFound() error {
	return mapped(CodeGitHubBindingNotFound, nil)
}

func GitHubProviderFailed() error {
	return mapped(CodeGitHubProviderFailed, nil)
}

func GitHubSubmissionInvalid() error {
	return mapped(CodeGitHubSubmissionInvalid, nil)
}

func GitHubSubmissionUnauthorized() error {
	return mapped(CodeGitHubSubmissionUnauthorized, nil)
}

// OAuthEmailConflict: the provider's (unverified) email collides with an
// existing local account, so we refuse to auto-link.
func OAuthEmailConflict(email string) error {
	return mapped(CodeOAuthEmailConflict, map[string]any{"email": email})
}

// OAuthNoEmail: the provider returned no email, so we can neither link nor register.
func OAuthNoEmail() error {
	return mapped(CodeOAuthNoEmail, nil)
}

// OAuthFailed is a generic provider/exchange failure for any non-redirect caller
// that needs a coded error (the redirect endpoints surface errors via query string).
func OAuthFailed() error {
	return mapped(CodeOAuthFailed, nil)
}

// VerificationInvalid: a verify/reset token is missing, expired, already used, or
// scoped to a different purpose (intentionally generic — no detail leak).
func VerificationInvalid() error {
	return mapped(CodeVerificationInvalid, nil)
}

// ResetThrottled: too many password-reset requests for this account/IP.
func ResetThrottled() error {
	return mapped(CodeResetThrottled, nil)
}

func ResetThrottledUntil(retryAt time.Time) error {
	if retryAt.IsZero() {
		return ResetThrottled()
	}
	return mapped(CodeResetThrottled, retryAtParams(retryAt))
}

// VerifyThrottled: too many email-verification requests for this account/IP.
func VerifyThrottled() error {
	return mapped(CodeVerifyThrottled, nil)
}

func VerifyThrottledUntil(retryAt time.Time) error {
	if retryAt.IsZero() {
		return VerifyThrottled()
	}
	return mapped(CodeVerifyThrottled, retryAtParams(retryAt))
}

func retryAtParams(retryAt time.Time) map[string]any {
	return map[string]any{"retryAt": retryAt.UTC().Format(time.RFC3339Nano)}
}

// Forbidden: the caller is authenticated but lacks the required privilege
// (e.g. a non-admin hitting an admin-only endpoint).
func Forbidden() error {
	return mapped(CodeForbidden, nil)
}

// IdentityNotFound: the target identity does not exist (admin operations on a
// user id that matches no row). 404.
func IdentityNotFound() error {
	return mapped(CodeIdentityNotFound, nil)
}

// InvalidStatus: an admin status change requested a value outside the lifecycle
// set {active, disabled, deleted}. 400.
func InvalidStatus(status string) error {
	return mapped(CodeInvalidStatus, map[string]any{"status": status})
}

// SelfAdminTarget: an admin attempted a destructive action (ban / delete /
// demote) against their own account. Refused to prevent self-lockout. 403.
func SelfAdminTarget() error {
	return mapped(CodeSelfAdminActionForbidden, nil)
}

// UnknownRole: the requested role slug is not in the fixed catalog (migration
// 0006 seeds {user, admin}). A client error, not a server fault — so it maps to
// 400 rather than leaking through the envelope as a generic 500.
func UnknownRole(slug string) error {
	return mapped(CodeUnknownRole, map[string]any{"role": slug})
}

// InvalidProfileReason is a stable machine-readable reason carried in
// identity.invalid_profile.params.reason. Keep these values backward-compatible:
// Account uses them to choose field-specific copy without inspecting prose.
type InvalidProfileReason string

const (
	ProfileReasonDisplayNameRequired InvalidProfileReason = "display_name_required"
	ProfileReasonFileRequired        InvalidProfileReason = "file_required"
	ProfileReasonImageTooLarge       InvalidProfileReason = "image_too_large"
	ProfileReasonUnsupportedImage    InvalidProfileReason = "unsupported_image"
	ProfileReasonUploadUnreadable    InvalidProfileReason = "upload_unreadable"
)

// InvalidProfile reports which profile validation rule failed through a stable
// enum rather than an implementation-detail message.
func InvalidProfile(reason InvalidProfileReason) error {
	return mapped(CodeInvalidProfile, map[string]any{"reason": string(reason)})
}

// SessionNotFound: the target session does not exist OR is not owned by the
// caller (intentionally merged — don't reveal another account's sessions).
func SessionNotFound() error {
	return mapped(CodeSessionNotFound, nil)
}

// CredentialConflict: the external account is already linked to a different identity.
func CredentialConflict(provider string) error {
	return mapped(CodeCredentialConflict, map[string]any{"provider": provider})
}

// CredentialNotFound: the caller has no such credential to unbind.
func CredentialNotFound() error {
	return mapped(CodeCredentialNotFound, nil)
}

// LastCredential: refusing to remove the only remaining login credential.
func LastCredential() error {
	return mapped(CodeLastCredential, nil)
}

// PasswordAlreadySet: SetPassword is for accounts WITHOUT a password (e.g.
// OAuth-only). An account that already has one must use ChangePassword.
func PasswordAlreadySet() error {
	return mapped(CodePasswordAlreadySet, nil)
}

// ── Personal Access Token (PAT) codes ───────────────────────────────────────

func PATNameRequired() error {
	return mapped(CodePATNameRequired, nil)
}

func PATScopesRequired() error {
	return mapped(CodePATScopesRequired, nil)
}

func PATScopeInvalid(index int) error {
	return mapped(CodePATScopeInvalid, map[string]any{
		"reason": "invalid_scope",
		"index":  index,
	})
}

func PATScopesTooMany(max int) error {
	return mapped(CodePATScopeInvalid, map[string]any{
		"reason": "too_many_scopes",
		"max":    max,
	})
}

func PATLimitReached(max int) error {
	return mapped(CodePATLimitReached, map[string]any{"max": max})
}

func PATNotFound() error {
	return mapped(CodePATNotFound, nil)
}

// PATInvalid is intentionally generic (no enumeration of why the token is bad).
func PATInvalid() error {
	return mapped(CodePATInvalid, nil)
}

func PATExpired() error {
	return mapped(CodePATExpired, nil)
}
