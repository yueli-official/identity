package mailer

import (
	"context"
	"testing"

	"platform/gokit/notificationclient"
)

type captureNotification struct {
	inputs []notificationclient.SendInput
}

func (capture *captureNotification) Send(_ context.Context, input notificationclient.SendInput) (notificationclient.SendOutput, error) {
	capture.inputs = append(capture.inputs, input)
	return notificationclient.SendOutput{MessageID: "message-1", Status: "pending"}, nil
}

func TestNotificationMailerUsesTypedIdentityScenes(t *testing.T) {
	capture := &captureNotification{}
	mailer := NewNotification(capture)
	if err := mailer.SendVerifyEmail(context.Background(), "reader@example.com", "https://account.test/verify?t=1"); err != nil {
		t.Fatal(err)
	}
	if err := mailer.SendPasswordReset(context.Background(), "reader@example.com", "https://account.test/reset?t=2"); err != nil {
		t.Fatal(err)
	}
	if len(capture.inputs) != 2 {
		t.Fatalf("inputs = %+v", capture.inputs)
	}
	if capture.inputs[0].Scene != "identity.verify_email" || capture.inputs[1].Scene != "identity.password_reset" {
		t.Fatalf("scenes = %q, %q", capture.inputs[0].Scene, capture.inputs[1].Scene)
	}
	if capture.inputs[0].IdempotencyKey == capture.inputs[1].IdempotencyKey ||
		capture.inputs[0].Data["actionUrl"] == "" || capture.inputs[0].Recipient.Email != "reader@example.com" {
		t.Fatalf("inputs = %+v", capture.inputs)
	}
}
