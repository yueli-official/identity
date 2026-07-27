// Package iderr declares Identity's immutable public Problem descriptors.
// Foundation owns validation and wire mapping; Identity owns its codes,
// statuses, parameters and public type URIs.
package iderr

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/yueli-official/foundation/go/problem"
)

const (
	CodeEmailTaken                   = "identity.email_taken"
	CodeInvalidCredentials           = "identity.invalid_credentials"
	CodeAccountDisabled              = "identity.account_disabled"
	CodeAccountLocked                = "identity.account_locked"
	CodeWeakPassword                 = "identity.weak_password"
	CodeInvalidEmail                 = "identity.invalid_email"
	CodeNotAuthenticated             = "identity.not_authenticated"
	CodeStepUpRequired               = "identity.step_up_required"
	CodeChallengeRequired            = "identity.challenge_required"
	CodeAbuseUnavailable             = "identity.abuse_unavailable"
	CodeAbuseReplay                  = "identity.abuse_attempt_replayed"
	CodeOAuthEmailConflict           = "identity.oauth_email_conflict"
	CodeOAuthNoEmail                 = "identity.oauth_no_email"
	CodeOAuthFailed                  = "identity.oauth_failed"
	CodeVerificationInvalid          = "identity.verification_invalid"
	CodeResetThrottled               = "identity.reset_throttled"
	CodeVerifyThrottled              = "identity.verify_throttled"
	CodeForbidden                    = "identity.forbidden"
	CodeSelfAdminActionForbidden     = "identity.self_admin_action_forbidden"
	CodeUnknownRole                  = "identity.unknown_role"
	CodeInvalidProfile               = "identity.invalid_profile"
	CodeSessionNotFound              = "identity.session_not_found"
	CodeCredentialConflict           = "identity.credential_conflict"
	CodeCredentialNotFound           = "identity.credential_not_found"
	CodeLastCredential               = "identity.last_credential"
	CodePasswordAlreadySet           = "identity.password_already_set"
	CodeIdentityNotFound             = "identity.not_found"
	CodeInvalidStatus                = "identity.invalid_status"
	CodeCapabilityNotFound           = "identity.capability_not_found"
	CodeProviderNotFound             = "identity.provider_not_found"
	CodeCapabilityProbeRateLimit     = "identity.capability_probe_rate_limited"
	CodeCapabilityAudit              = "identity.capability_audit_unavailable"
	CodeInvalidGuestRequest          = "identity.guest_request_invalid"
	CodeInvalidGuestSession          = "identity.guest_session_invalid"
	CodeInvalidGuestAudience         = "identity.guest_audience_invalid"
	CodeGuestClaimConflict           = "identity.guest_claim_conflict"
	CodePasskeyUnavailable           = "identity.passkey_unavailable"
	CodePasskeyCeremonyInvalid       = "identity.passkey_ceremony_invalid"
	CodePasskeyExists                = "identity.passkey_exists"
	CodeMFAUnavailable               = "identity.mfa_unavailable"
	CodeTOTPEnrollmentInvalid        = "identity.totp_enrollment_invalid"
	CodeTOTPCodeInvalid              = "identity.totp_code_invalid"
	CodeTOTPNotFound                 = "identity.totp_not_found"
	CodeMFATransactionInvalid        = "identity.mfa_transaction_invalid"
	CodeRecoveryCodeInvalid          = "identity.recovery_code_invalid"
	CodeStepUpRequestInvalid         = "identity.step_up_request_invalid"
	CodeStepUpMethodUnavailable      = "identity.step_up_method_unavailable"
	CodeStepUpProofInvalid           = "identity.step_up_proof_invalid"
	CodeStepUpProofReplayed          = "identity.step_up_proof_replayed"
	CodePublisherConsumerNotFound    = "identity.publisher_consumer_not_found"
	CodePublisherConsumerDisabled    = "identity.publisher_consumer_disabled"
	CodePublisherAttestationInvalid  = "identity.publisher_attestation_invalid"
	CodePublisherIdempotencyConflict = "identity.publisher_idempotency_conflict"
	CodePublisherSigningUnavailable  = "identity.publisher_signing_unavailable"
	CodePublisherRotationPending     = "identity.publisher_rotation_pending"
	CodePublisherKeyTransition       = "identity.publisher_key_transition_invalid"
	CodePublisherTrustInvalid        = "identity.publisher_trust_manifest_invalid"
	CodePublisherRootUntrusted       = "identity.publisher_root_untrusted"
	CodeGitHubBindingUnavailable     = "identity.github_binding_unavailable"
	CodeGitHubBindingAttemptInvalid  = "identity.github_binding_attempt_invalid"
	CodeGitHubBindingConflict        = "identity.github_binding_conflict"
	CodeGitHubBindingNotFound        = "identity.github_binding_not_found"
	CodeGitHubProviderFailed         = "identity.github_provider_failed"
	CodeGitHubSubmissionInvalid      = "identity.github_submission_invalid"
	CodeGitHubSubmissionUnauthorized = "identity.github_submission_unauthorized"
	CodePATNameRequired              = "identity.pat_name_required"
	CodePATScopesRequired            = "identity.pat_scopes_required"
	CodePATScopeInvalid              = "identity.pat_scope_invalid"
	CodePATLimitReached              = "identity.pat_limit_reached"
	CodePATNotFound                  = "identity.pat_not_found"
	CodePATInvalid                   = "identity.pat_invalid"
	CodePATExpired                   = "identity.pat_expired"
)

var (
	DescriptorUnauthorized = descriptor("common.unauthorized", http.StatusUnauthorized)
	DescriptorRateLimited  = descriptor("common.rate_limited", http.StatusTooManyRequests)
	DescriptorValidation   = descriptor("common.validation_failed", http.StatusBadRequest)
	DescriptorInternal     = descriptor("common.internal", http.StatusInternalServerError)

	descriptors = map[string]problem.Descriptor{
		CodeEmailTaken:                   descriptor(CodeEmailTaken, http.StatusConflict),
		CodeInvalidCredentials:           descriptor(CodeInvalidCredentials, http.StatusUnauthorized),
		CodeAccountDisabled:              descriptor(CodeAccountDisabled, http.StatusForbidden),
		CodeAccountLocked:                descriptor(CodeAccountLocked, http.StatusTooManyRequests),
		CodeWeakPassword:                 descriptor(CodeWeakPassword, http.StatusBadRequest),
		CodeInvalidEmail:                 descriptor(CodeInvalidEmail, http.StatusBadRequest),
		CodeNotAuthenticated:             descriptor(CodeNotAuthenticated, http.StatusUnauthorized),
		CodeStepUpRequired:               descriptor(CodeStepUpRequired, http.StatusPreconditionRequired),
		CodeChallengeRequired:            descriptor(CodeChallengeRequired, http.StatusForbidden),
		CodeAbuseUnavailable:             descriptor(CodeAbuseUnavailable, http.StatusServiceUnavailable),
		CodeAbuseReplay:                  descriptor(CodeAbuseReplay, http.StatusConflict),
		CodeOAuthEmailConflict:           descriptor(CodeOAuthEmailConflict, http.StatusConflict),
		CodeOAuthNoEmail:                 descriptor(CodeOAuthNoEmail, http.StatusBadRequest),
		CodeOAuthFailed:                  descriptor(CodeOAuthFailed, http.StatusUnauthorized),
		CodeVerificationInvalid:          descriptor(CodeVerificationInvalid, http.StatusBadRequest),
		CodeResetThrottled:               descriptor(CodeResetThrottled, http.StatusTooManyRequests),
		CodeVerifyThrottled:              descriptor(CodeVerifyThrottled, http.StatusTooManyRequests),
		CodeForbidden:                    descriptor(CodeForbidden, http.StatusForbidden),
		CodeSelfAdminActionForbidden:     descriptor(CodeSelfAdminActionForbidden, http.StatusForbidden),
		CodeUnknownRole:                  descriptor(CodeUnknownRole, http.StatusBadRequest),
		CodeInvalidProfile:               descriptor(CodeInvalidProfile, http.StatusBadRequest),
		CodeSessionNotFound:              descriptor(CodeSessionNotFound, http.StatusNotFound),
		CodeCredentialConflict:           descriptor(CodeCredentialConflict, http.StatusConflict),
		CodeCredentialNotFound:           descriptor(CodeCredentialNotFound, http.StatusNotFound),
		CodeLastCredential:               descriptor(CodeLastCredential, http.StatusConflict),
		CodePasswordAlreadySet:           descriptor(CodePasswordAlreadySet, http.StatusConflict),
		CodeIdentityNotFound:             descriptor(CodeIdentityNotFound, http.StatusNotFound),
		CodeInvalidStatus:                descriptor(CodeInvalidStatus, http.StatusBadRequest),
		CodeCapabilityNotFound:           descriptor(CodeCapabilityNotFound, http.StatusNotFound),
		CodeProviderNotFound:             descriptor(CodeProviderNotFound, http.StatusNotFound),
		CodeCapabilityProbeRateLimit:     descriptor(CodeCapabilityProbeRateLimit, http.StatusTooManyRequests),
		CodeCapabilityAudit:              descriptor(CodeCapabilityAudit, http.StatusInternalServerError),
		CodeInvalidGuestRequest:          descriptor(CodeInvalidGuestRequest, http.StatusBadRequest),
		CodeInvalidGuestSession:          descriptor(CodeInvalidGuestSession, http.StatusUnauthorized),
		CodeInvalidGuestAudience:         descriptor(CodeInvalidGuestAudience, http.StatusForbidden),
		CodeGuestClaimConflict:           descriptor(CodeGuestClaimConflict, http.StatusConflict),
		CodePasskeyUnavailable:           descriptor(CodePasskeyUnavailable, http.StatusServiceUnavailable),
		CodePasskeyCeremonyInvalid:       descriptor(CodePasskeyCeremonyInvalid, http.StatusBadRequest),
		CodePasskeyExists:                descriptor(CodePasskeyExists, http.StatusConflict),
		CodeMFAUnavailable:               descriptor(CodeMFAUnavailable, http.StatusServiceUnavailable),
		CodeTOTPEnrollmentInvalid:        descriptor(CodeTOTPEnrollmentInvalid, http.StatusBadRequest),
		CodeTOTPCodeInvalid:              descriptor(CodeTOTPCodeInvalid, http.StatusUnauthorized),
		CodeTOTPNotFound:                 descriptor(CodeTOTPNotFound, http.StatusNotFound),
		CodeMFATransactionInvalid:        descriptor(CodeMFATransactionInvalid, http.StatusBadRequest),
		CodeRecoveryCodeInvalid:          descriptor(CodeRecoveryCodeInvalid, http.StatusUnauthorized),
		CodeStepUpRequestInvalid:         descriptor(CodeStepUpRequestInvalid, http.StatusBadRequest),
		CodeStepUpMethodUnavailable:      descriptor(CodeStepUpMethodUnavailable, http.StatusConflict),
		CodeStepUpProofInvalid:           descriptor(CodeStepUpProofInvalid, http.StatusUnauthorized),
		CodeStepUpProofReplayed:          descriptor(CodeStepUpProofReplayed, http.StatusConflict),
		CodePublisherConsumerNotFound:    descriptor(CodePublisherConsumerNotFound, http.StatusNotFound),
		CodePublisherConsumerDisabled:    descriptor(CodePublisherConsumerDisabled, http.StatusConflict),
		CodePublisherAttestationInvalid:  descriptor(CodePublisherAttestationInvalid, http.StatusBadRequest),
		CodePublisherIdempotencyConflict: descriptor(CodePublisherIdempotencyConflict, http.StatusConflict),
		CodePublisherSigningUnavailable:  descriptor(CodePublisherSigningUnavailable, http.StatusServiceUnavailable),
		CodePublisherRotationPending:     descriptor(CodePublisherRotationPending, http.StatusConflict),
		CodePublisherKeyTransition:       descriptor(CodePublisherKeyTransition, http.StatusConflict),
		CodePublisherTrustInvalid:        descriptor(CodePublisherTrustInvalid, http.StatusBadRequest),
		CodePublisherRootUntrusted:       descriptor(CodePublisherRootUntrusted, http.StatusBadRequest),
		CodeGitHubBindingUnavailable:     descriptor(CodeGitHubBindingUnavailable, http.StatusServiceUnavailable),
		CodeGitHubBindingAttemptInvalid:  descriptor(CodeGitHubBindingAttemptInvalid, http.StatusBadRequest),
		CodeGitHubBindingConflict:        descriptor(CodeGitHubBindingConflict, http.StatusConflict),
		CodeGitHubBindingNotFound:        descriptor(CodeGitHubBindingNotFound, http.StatusNotFound),
		CodeGitHubProviderFailed:         descriptor(CodeGitHubProviderFailed, http.StatusBadGateway),
		CodeGitHubSubmissionInvalid:      descriptor(CodeGitHubSubmissionInvalid, http.StatusBadRequest),
		CodeGitHubSubmissionUnauthorized: descriptor(CodeGitHubSubmissionUnauthorized, http.StatusForbidden),
		CodePATNameRequired:              descriptor(CodePATNameRequired, http.StatusBadRequest),
		CodePATScopesRequired:            descriptor(CodePATScopesRequired, http.StatusBadRequest),
		CodePATScopeInvalid:              descriptor(CodePATScopeInvalid, http.StatusBadRequest),
		CodePATLimitReached:              descriptor(CodePATLimitReached, http.StatusConflict),
		CodePATNotFound:                  descriptor(CodePATNotFound, http.StatusNotFound),
		CodePATInvalid:                   descriptor(CodePATInvalid, http.StatusUnauthorized),
		CodePATExpired:                   descriptor(CodePATExpired, http.StatusUnauthorized),
	}
)

func descriptor(code string, status int) problem.Descriptor {
	return problem.MustDescriptor(
		problem.MustKind(code, status),
		"https://errors.yueli.dev/problems/"+code,
	)
}

// DescriptorForCode resolves one Identity-owned public error contract.
func DescriptorForCode(code string) (problem.Descriptor, bool) {
	value, ok := descriptors[code]
	return value, ok
}

// CatalogEntry 是供 Account 与契约工具消费的稳定机器可读投影。
type CatalogEntry struct {
	Code   string `json:"code"`
	Status int    `json:"status"`
}

// Catalog 返回按错误码排序的 Identity 公共错误合同副本。
func Catalog() []CatalogEntry {
	result := make([]CatalogEntry, 0, len(descriptors))
	for code, value := range descriptors {
		result = append(result, CatalogEntry{
			Code:   code,
			Status: value.Kind().Status(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})
	return result
}

// Resolve exposes the immutable public mapping for service logic and tests
// without depending on an HTTP framework.
func Resolve(err error) (problem.Problem, bool) {
	value, ok, resolveErr := problem.FromError(err, "identity-error-inspection")
	return value, ok && resolveErr == nil
}

func mapped(code string, params problem.Parameters) error {
	value, ok := DescriptorForCode(code)
	if !ok {
		return fmt.Errorf("identity public error code is not declared: %s", code)
	}
	result, err := problem.NewError(value, params)
	if err != nil {
		return fmt.Errorf("identity public error %s: %w", code, err)
	}
	return result
}
