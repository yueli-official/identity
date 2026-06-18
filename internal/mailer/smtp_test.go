package mailer

import (
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	m := NewSMTP("h", "465", "u", "p", "from@x.com", "用户中心")
	msg := string(m.buildMessage("to@y.com", "Sub", "<b>hi</b>"))
	for _, want := range []string{
		"From: 用户中心 <from@x.com>",
		"To: to@y.com",
		"Subject: Sub",
		"Content-Type: text/html; charset=UTF-8",
		"<b>hi</b>",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q in:\n%s", want, msg)
		}
	}
}
