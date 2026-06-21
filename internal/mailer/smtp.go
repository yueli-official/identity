package mailer

import (
	"context"
	"fmt"

	"platform/gokit/mail"
)

// SMTPMailer renders identity's transactional emails (verify / reset) and
// delivers them via the shared gokit/mail transport. The SMTP transport itself
// lives in gokit/mail (lifted there when the blog became the second consumer).
type SMTPMailer struct {
	sender mail.Sender
}

func NewSMTP(host, port, username, password, from, fromName string) *SMTPMailer {
	return &SMTPMailer{sender: mail.NewSMTP(host, port, username, password, from, fromName)}
}

func (m *SMTPMailer) SendVerifyEmail(ctx context.Context, to, link string) error {
	return m.sender.Send(ctx, to, "验证你的邮箱",
		fmt.Sprintf("<p>请点击以下链接验证你的邮箱地址：</p><p><a href=\"%s\">%s</a></p><p>链接 24 小时内有效。</p>", link, link))
}

func (m *SMTPMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	return m.sender.Send(ctx, to, "重置你的密码",
		fmt.Sprintf("<p>我们收到了重置密码的请求。点击以下链接设置新密码：</p><p><a href=\"%s\">%s</a></p><p>链接 1 小时内有效。若非本人操作请忽略。</p>", link, link))
}

var (
	_ Mailer = (*SMTPMailer)(nil)
	_ Mailer = (*DevMailer)(nil)
)
