package mailer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"platform/gokit/notificationclient"
)

type notificationSender interface {
	Send(context.Context, notificationclient.SendInput) (notificationclient.SendOutput, error)
}

// NotificationMailer adapts Identity's narrow mailer interface to the shared
// Notification service. Identity owns action-token generation; Notification
// owns templates, provider routing, retry, suppression, and delivery records.
type NotificationMailer struct {
	client notificationSender
}

func NewNotification(client notificationSender) *NotificationMailer {
	return &NotificationMailer{client: client}
}

func (mailer *NotificationMailer) SendVerifyEmail(ctx context.Context, to, link string) error {
	return mailer.send(ctx, "identity.verify_email", to, link)
}

func (mailer *NotificationMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	return mailer.send(ctx, "identity.password_reset", to, link)
}

func (mailer *NotificationMailer) SendSecurityAlert(
	ctx context.Context,
	alert SecurityAlert,
) error {
	if mailer == nil || mailer.client == nil {
		return fmt.Errorf("identity notification client is unavailable")
	}
	idempotencyKey := "identity-security:" + alert.EventID
	if alert.EventID == "" {
		sum := sha256.Sum256([]byte(
			alert.To + "\x00" + alert.Action + "\x00" + alert.OccurredAt.String(),
		))
		idempotencyKey = fmt.Sprintf("identity-security:%x", sum[:])
	}
	occurredAt := alert.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	_, err := mailer.client.Send(ctx, notificationclient.SendInput{
		IdempotencyKey: idempotencyKey,
		Scene:          "identity.security_alert",
		Channel:        "email",
		Recipient:      notificationclient.Recipient{Email: alert.To},
		Data: map[string]string{
			"action": alert.Action, "device": alert.Device, "ip": alert.IP,
			"occurredAt": occurredAt.Format(time.RFC3339),
			"accountUrl": alert.AccountURL,
		},
	})
	return err
}

func (mailer *NotificationMailer) send(ctx context.Context, scene, to, link string) error {
	if mailer == nil || mailer.client == nil {
		return fmt.Errorf("identity notification client is unavailable")
	}
	sum := sha256.Sum256([]byte(scene + "\x00" + to + "\x00" + link))
	_, err := mailer.client.Send(ctx, notificationclient.SendInput{
		IdempotencyKey: fmt.Sprintf("%s:%x", scene, sum[:]),
		Scene:          scene,
		Channel:        "email",
		Recipient:      notificationclient.Recipient{Email: to},
		Data:           map[string]string{"actionUrl": link},
	})
	return err
}

var _ Mailer = (*NotificationMailer)(nil)
var _ SecurityNotifier = (*NotificationMailer)(nil)
