package identitysecurity

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/mailer"
	"platform/services/identity/internal/repo"
)

type Sink struct {
	audit      repo.AuditRepo
	identities repo.IdentityRepo
	notifier   mailer.SecurityNotifier
	accountURL string
}

func New(
	audit repo.AuditRepo,
	identities repo.IdentityRepo,
	notifier mailer.SecurityNotifier,
	accountURL string,
) *Sink {
	return &Sink{
		audit: audit, identities: identities, notifier: notifier,
		accountURL: accountURL,
	}
}

func (sink *Sink) RecordAuthenticationEvent(
	ctx context.Context,
	event authentication.SecurityEvent,
) {
	requestActor := actor.From(ctx)
	if sink.audit != nil {
		if err := sink.audit.InsertAudit(ctx, repo.AuditRow{
			Event: string(event.Kind), ActorID: event.IdentityID,
			TargetID: event.IdentityID, IP: requestActor.IP,
			UserAgent: requestActor.UserAgent, RequestID: requestActor.RequestID,
			Result: "success", Detail: event.Detail, OccurredAt: event.OccurredAt,
		}); err != nil {
			g.Log().Errorf(ctx, "authentication audit %s failed: %v", event.Kind, err)
		}
	}
	if !notificationEvent(event.Kind) {
		return
	}
	if sink.identities == nil || sink.notifier == nil {
		return
	}
	identity, err := sink.identities.GetByID(ctx, event.IdentityID)
	if err != nil {
		g.Log().Errorf(ctx, "authentication security recipient lookup failed: %v", err)
		return
	}
	action := securityAction(event.Kind)
	if err := sink.notifier.SendSecurityAlert(ctx, mailer.SecurityAlert{
		EventID: event.ID, To: identity.Email, Action: action,
		Device: requestActor.UserAgent, IP: requestActor.IP,
		OccurredAt: event.OccurredAt, AccountURL: sink.accountURL,
	}); err != nil {
		g.Log().Errorf(ctx, "authentication security notification %s failed: %v", event.Kind, err)
	}
}

func notificationEvent(kind authentication.SecurityEventKind) bool {
	switch kind {
	case authentication.EventPasskeyRegistered,
		authentication.EventPasskeyRevoked,
		authentication.EventTOTPEnrolled,
		authentication.EventTOTPRevoked,
		authentication.EventRecoveryGenerated,
		authentication.EventRecoveryUsed:
		return true
	default:
		return false
	}
}

func securityAction(kind authentication.SecurityEventKind) string {
	switch kind {
	case authentication.EventPasskeyRegistered:
		return "添加了新的通行密钥"
	case authentication.EventPasskeyRevoked:
		return "移除了通行密钥"
	case authentication.EventTOTPEnrolled:
		return "启用了身份验证器动态口令"
	case authentication.EventTOTPRevoked:
		return "移除了身份验证器动态口令"
	case authentication.EventRecoveryGenerated:
		return "重新生成了恢复代码"
	case authentication.EventRecoveryUsed:
		return "使用了恢复代码登录"
	default:
		return "更改了账户安全设置"
	}
}

var _ authentication.SecurityEventSink = (*Sink)(nil)
