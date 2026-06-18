package mailer

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// DevMailer is the default mailer when SMTP is unconfigured. It logs the action
// link so local/dev flows are testable without a real inbox.
type DevMailer struct{}

func NewDev() *DevMailer { return &DevMailer{} }

func (DevMailer) SendVerifyEmail(ctx context.Context, to, link string) error {
	g.Log().Infof(ctx, "[mailer:dev] verify-email to=%s link=%s", to, link)
	return nil
}

func (DevMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	g.Log().Infof(ctx, "[mailer:dev] password-reset to=%s link=%s", to, link)
	return nil
}
