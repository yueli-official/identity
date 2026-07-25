package logic

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/githubbinding"
	"platform/services/identity/internal/publisher"
	"platform/services/identity/internal/repo"
)

// Audit event names (dotted).
const (
	EvLoginSuccess         = "login.success"
	EvLoginFailure         = "login.failure"
	EvLogout               = "logout"
	EvLogoutAll            = "logout.all"
	EvRegister             = "identity.register"
	EvOAuthLogin           = "oauth.login"
	EvCredentialLinked     = "credential.oauth_linked"
	EvPwResetRequested     = "password.reset_requested"
	EvPwReset              = "password.reset"
	EvEmailVerifyRequested = "email.verification_requested"
	EvEmailVerified        = "email.verified"
	EvRoleGranted          = "role.granted"
	EvRoleRevoked          = "role.revoked"
	EvRoleDefaultGranted   = "role.default_granted"

	EvPATCreated = "pat.created"
	EvPATRevoked = "pat.revoked"

	EvProfileUpdated     = "profile.updated"
	EvPasswordChanged    = "password.changed"
	EvPasswordSet        = "password.set"
	EvSessionRevoked     = "session.revoked"
	EvCredentialUnlinked = "credential.oauth_unlinked"

	EvAdminUserCreated   = "admin.user_created"
	EvAdminStatusChanged = "admin.user_status_changed"
	EvAdminUserDeleted   = "admin.user_deleted"
	EvAdminPasswordReset = "admin.password_reset"

	EvPublisherAttestationIssued = "publisher.attestation_issued"
	EvPublisherKeyPrepared       = "publisher.key_prepared"
	EvPublisherManifestApplied   = "publisher.manifest_applied"

	EvGitHubBindingVerified      = "github.binding_verified"
	EvGitHubBindingRenamed       = "github.binding_login_refreshed"
	EvGitHubBindingUnbound       = "github.binding_unbound"
	EvGitHubAuthorizationRevoked = "github.authorization_revoked"
	EvGitHubBindingFailed        = "github.binding_failed"
	EvGitHubSubmissionAuthorized = "github.submission_authorized"
	EvGitHubSubmissionDenied     = "github.submission_denied"
)

// AuditEvent is the logic-layer description of one auditable event. IP/UA/request-id
// are merged from ctx by audit(); callers only supply the semantic fields.
type AuditEvent struct {
	Event    string
	ActorID  string
	TargetID string
	Email    string
	ClientID string
	Result   string // "" → "success"
	Detail   map[string]any
}

func (s *Service) RecordPublisherKeyPrepared(
	ctx context.Context,
	adminID string,
	key publisher.VerificationKey,
) {
	s.audit(ctx, AuditEvent{
		Event: EvPublisherKeyPrepared, ActorID: adminID,
		Detail: map[string]any{
			"key_id": key.KeyID, "algorithm": key.Algorithm,
			"purpose": key.Purpose, "status": key.Status,
		},
	})
}

func (s *Service) RecordPublisherManifestApplied(
	ctx context.Context,
	adminID string,
	value publisher.VerifiedTrustManifest,
	activeKeyID string,
) {
	s.audit(ctx, AuditEvent{
		Event: EvPublisherManifestApplied, ActorID: adminID,
		Detail: map[string]any{
			"manifest_version": value.Manifest.ManifestVersion,
			"snapshot_hash":    value.SnapshotHash,
			"root_key_id":      value.Manifest.RootKeyID,
			"active_key_id":    activeKeyID,
			"signing_enabled":  activeKeyID != "",
		},
	})
}

// QueryAudit is a thin read-only passthrough to the store so the controller
// layer can serve admin audit-log queries without bypassing the service seam.
func (s *Service) QueryAudit(ctx context.Context, f repo.AuditFilter) ([]repo.AuditRow, error) {
	return s.store.QueryAudit(ctx, f)
}

// audit records an event best-effort: actor IP/UA are merged from ctx, and a write
// failure is logged and swallowed (it must never break the business operation).
func (s *Service) audit(ctx context.Context, e AuditEvent) {
	ac := actor.From(ctx)
	result := e.Result
	if result == "" {
		result = "success"
	}
	row := repo.AuditRow{
		Event:      e.Event,
		ActorID:    e.ActorID,
		TargetID:   e.TargetID,
		ActorEmail: e.Email,
		IP:         ac.IP,
		UserAgent:  ac.UserAgent,
		ClientID:   e.ClientID,
		RequestID:  ac.RequestID,
		Result:     result,
		Detail:     e.Detail,
	}
	if err := s.store.InsertAudit(ctx, row); err != nil {
		g.Log().Errorf(ctx, "audit: record %s failed: %v", e.Event, err)
	}
}

func (s *Service) RecordPublisherAttestation(
	ctx context.Context,
	identityID string,
	value publisher.Attestation,
) {
	s.audit(ctx, AuditEvent{
		Event:   EvPublisherAttestationIssued,
		ActorID: identityID, TargetID: identityID, ClientID: value.Audience,
		Detail: map[string]any{
			"attestation_id":    value.AttestationID,
			"statement_digest":  value.StatementDigest,
			"consumer_instance": value.ConsumerInstance,
			"namespace":         value.Namespace,
			"artifact_kind":     value.Artifact.Kind,
			"artifact_identity": value.Artifact.Identity,
			"artifact_version":  value.Artifact.Version,
			"key_id":            value.KeyID,
		},
	})
}

func (s *Service) RecordGitHubBindingVerified(
	ctx context.Context,
	binding githubbinding.Binding,
	created bool,
	renamed bool,
) {
	event := EvGitHubBindingVerified
	if renamed {
		event = EvGitHubBindingRenamed
	}
	s.audit(ctx, AuditEvent{
		Event: event, ActorID: binding.IdentityID, TargetID: binding.IdentityID,
		Detail: map[string]any{
			"binding_id": binding.ID, "provider": binding.Provider,
			"provider_account_id": binding.ProviderAccountID,
			"login_snapshot":      binding.Login, "created": created,
			"renamed": renamed,
		},
	})
}

func (s *Service) RecordGitHubBindingUnbound(
	ctx context.Context,
	binding githubbinding.Binding,
) {
	s.audit(ctx, AuditEvent{
		Event:   EvGitHubBindingUnbound,
		ActorID: binding.IdentityID, TargetID: binding.IdentityID,
		Detail: map[string]any{
			"binding_id":          binding.ID,
			"provider_account_id": binding.ProviderAccountID,
			"login_snapshot":      binding.Login,
		},
	})
}

func (s *Service) RecordGitHubAuthorizationRevoked(
	ctx context.Context,
	binding githubbinding.Binding,
	deliveryID string,
) {
	s.audit(ctx, AuditEvent{
		Event: EvGitHubAuthorizationRevoked, TargetID: binding.IdentityID,
		Detail: map[string]any{
			"binding_id":          binding.ID,
			"provider_account_id": binding.ProviderAccountID,
			"login_snapshot":      binding.Login, "delivery_id": deliveryID,
		},
	})
}

func (s *Service) RecordGitHubBindingFailure(ctx context.Context, reason string) {
	s.audit(ctx, AuditEvent{
		Event: EvGitHubBindingFailed, Result: "failure",
		Detail: map[string]any{"reason": reason},
	})
}

func (s *Service) RecordGitHubSubmissionAuthorized(
	ctx context.Context,
	clientID string,
	publisherSubject string,
	bindingID string,
	manifestDigest string,
	repositoryID string,
	pullRequestNumber int64,
	headCommitSHA string,
) {
	s.audit(ctx, AuditEvent{
		Event: EvGitHubSubmissionAuthorized, TargetID: publisherSubject,
		ClientID: clientID,
		Detail: map[string]any{
			"binding_id": bindingID, "manifest_digest": manifestDigest,
			"repository_id":       repositoryID,
			"pull_request_number": pullRequestNumber,
			"head_commit_sha":     headCommitSHA,
		},
	})
}

func (s *Service) RecordGitHubSubmissionDenied(
	ctx context.Context,
	clientID string,
	reason string,
	accountID string,
	repositoryID string,
	pullRequestNumber int64,
) {
	s.audit(ctx, AuditEvent{
		Event: EvGitHubSubmissionDenied, ClientID: clientID, Result: "failure",
		Detail: map[string]any{
			"reason": reason, "provider_account_id": accountID,
			"repository_id":       repositoryID,
			"pull_request_number": pullRequestNumber,
		},
	})
}
