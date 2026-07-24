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
	if event.Kind != authentication.EventPasskeyRegistered &&
		event.Kind != authentication.EventPasskeyRevoked {
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
	action := "添加了新的通行密钥"
	if event.Kind == authentication.EventPasskeyRevoked {
		action = "移除了通行密钥"
	}
	if err := sink.notifier.SendSecurityAlert(ctx, mailer.SecurityAlert{
		EventID: event.ID, To: identity.Email, Action: action,
		Device: requestActor.UserAgent, IP: requestActor.IP,
		OccurredAt: event.OccurredAt, AccountURL: sink.accountURL,
	}); err != nil {
		g.Log().Errorf(ctx, "authentication security notification %s failed: %v", event.Kind, err)
	}
}

var _ authentication.SecurityEventSink = (*Sink)(nil)
