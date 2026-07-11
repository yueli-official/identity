package logic

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"platform/services/identity/internal/actor"
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
