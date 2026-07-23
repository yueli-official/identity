package mailer

import (
	"context"
	"crypto/sha256"
	"fmt"

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
