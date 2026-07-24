package identitysecurity

import (
	"context"
	"testing"
	"time"

	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/mailer"
	"platform/services/identity/internal/repo"
)

type captureSecurityNotifier struct {
	alerts []mailer.SecurityAlert
}

func (capture *captureSecurityNotifier) SendSecurityAlert(
	_ context.Context,
	alert mailer.SecurityAlert,
) error {
	capture.alerts = append(capture.alerts, alert)
	return nil
}

func TestSinkAuditsEveryEventButNotifiesCredentialLifecycle(t *testing.T) {
	store := repo.NewMemory()
	ctx := actor.With(context.Background(), actor.Actor{
		IP: "192.0.2.9", UserAgent: "Test Browser", RequestID: "request-1",
	})
	identity, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "security@example.test", PasswordHash: "hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	notifier := &captureSecurityNotifier{}
	sink := New(store, store, notifier, "https://account.example.test")
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	sink.RecordAuthenticationEvent(ctx, authentication.SecurityEvent{
		ID: "event-register", Kind: authentication.EventPasskeyRegistered,
		IdentityID: identity.ID, CredentialID: "credential-1",
		OccurredAt: now, Detail: map[string]any{"attachment": "platform"},
	})
	sink.RecordAuthenticationEvent(ctx, authentication.SecurityEvent{
		ID: "event-login", Kind: authentication.EventPasskeyLogin,
		IdentityID: identity.ID, CredentialID: "credential-1",
		OccurredAt: now.Add(time.Minute),
	})

	rows, err := store.QueryAudit(ctx, repo.AuditFilter{IdentityID: identity.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Event != string(authentication.EventPasskeyLogin) ||
		rows[1].IP != "192.0.2.9" || rows[1].RequestID != "request-1" {
		t.Fatalf("audit rows = %+v", rows)
	}
	if len(notifier.alerts) != 1 {
		t.Fatalf("security alerts = %+v", notifier.alerts)
	}
	alert := notifier.alerts[0]
	if alert.EventID != "event-register" || alert.To != identity.Email ||
		alert.IP != "192.0.2.9" || alert.AccountURL == "" {
		t.Fatalf("security alert = %+v", alert)
	}
}

var _ mailer.SecurityNotifier = (*captureSecurityNotifier)(nil)
