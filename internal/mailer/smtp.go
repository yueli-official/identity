package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPMailer delivers over implicit TLS on :465 (tls.Dial -> smtp.NewClient ->
// PlainAuth). net/smtp.SendMail's STARTTLS path does not fit port 465.
type SMTPMailer struct {
	host, port, username, password, from, fromName string
}

func NewSMTP(host, port, username, password, from, fromName string) *SMTPMailer {
	return &SMTPMailer{host, port, username, password, from, fromName}
}

func (m *SMTPMailer) SendVerifyEmail(ctx context.Context, to, link string) error {
	return m.send(to, "验证你的邮箱",
		fmt.Sprintf("<p>请点击以下链接验证你的邮箱地址：</p><p><a href=\"%s\">%s</a></p><p>链接 24 小时内有效。</p>", link, link))
}

func (m *SMTPMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	return m.send(to, "重置你的密码",
		fmt.Sprintf("<p>我们收到了重置密码的请求。点击以下链接设置新密码：</p><p><a href=\"%s\">%s</a></p><p>链接 1 小时内有效。若非本人操作请忽略。</p>", link, link))
}

// buildMessage is pure (unit-tested); send dials and delivers.
func (m *SMTPMailer) buildMessage(to, subject, htmlBody string) []byte {
	from := m.from
	if m.fromName != "" {
		from = fmt.Sprintf("%s <%s>", m.fromName, m.from)
	}
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

func (m *SMTPMailer) send(to, subject, htmlBody string) error {
	addr := m.host + ":" + m.port
	tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host})
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	c, err := smtp.NewClient(tlsConn, m.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(m.from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(m.buildMessage(to, subject, htmlBody)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

var _ Mailer = (*SMTPMailer)(nil)
var _ Mailer = (*DevMailer)(nil)
