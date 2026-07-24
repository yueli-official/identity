package logic_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// capMailer captures the links handed to the mailer so tests can replay tokens.
type capMailer struct{ verifyLink, resetLink, verifyTo, resetTo string }

type capRevoker struct{ identities []string }

func (c *capRevoker) RevokeRefreshBySession(context.Context, string) error { return nil }
func (c *capRevoker) RevokeRefreshByIdentity(_ context.Context, identityID string) error {
	c.identities = append(c.identities, identityID)
	return nil
}

func (c *capMailer) SendVerifyEmail(_ context.Context, to, link string) error {
	c.verifyTo, c.verifyLink = to, link
	return nil
}
func (c *capMailer) SendPasswordReset(_ context.Context, to, link string) error {
	c.resetTo, c.resetLink = to, link
	return nil
}

func tokenFromLink(link string) string {
	u, _ := url.Parse(link)
	return u.Query().Get("token")
}

func TestVerifyEmail_Flow(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	cm := &capMailer{}
	s.SetMailer(cm)
	ctx := context.Background()
	id, _ := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "u@example.com", DisplayName: "U", PasswordHash: "h"})
	if err := s.RequestEmailVerification(ctx, id.ID, "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if cm.verifyTo != "u@example.com" || !strings.Contains(cm.verifyLink, "/verify-email?token=") {
		t.Fatalf("bad mail: %+v", cm)
	}
	if err := s.VerifyEmail(ctx, tokenFromLink(cm.verifyLink)); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetByID(ctx, id.ID)
	if !got.EmailVerified {
		t.Fatal("email not marked verified")
	}
	if err := s.VerifyEmail(ctx, tokenFromLink(cm.verifyLink)); err == nil {
		t.Fatal("token must be single-use")
	}
}

func TestPasswordReset_Flow_ForceLogout(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	cm := &capMailer{}
	s.SetMailer(cm)
	revoker := &capRevoker{}
	s.SetRefreshRevoker(revoker)
	ctx := context.Background()
	reg, _ := s.Register(ctx, logic.RegisterInput{Email: "r@example.com", Password: "oldpass12", DisplayName: "R"})
	// establish a session, then reset, then ensure it's gone
	login, _ := s.Login(ctx, logic.LoginInput{Email: "r@example.com", Password: "oldpass12"})
	if err := s.RequestPasswordReset(ctx, "r@example.com", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cm.resetLink, "/reset?token=") {
		t.Fatalf("no reset link: %q", cm.resetLink)
	}
	if err := s.ResetPassword(ctx, tokenFromLink(cm.resetLink), "newpass12"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetSession(ctx, login.SessionID); err == nil {
		t.Fatal("other sessions must be force-logged-out")
	}
	if len(revoker.identities) != 1 || revoker.identities[0] != reg.ID {
		t.Fatalf("refresh revocations = %v, want [%s]", revoker.identities, reg.ID)
	}
	if _, err := s.Login(ctx, logic.LoginInput{Email: "r@example.com", Password: "newpass12"}); err != nil {
		t.Fatal("new password must work")
	}
	if _, err := s.Login(ctx, logic.LoginInput{Email: "r@example.com", Password: "oldpass12"}); err == nil {
		t.Fatal("old password must fail")
	}
}

func TestPasswordReset_UnknownEmail_NoLeak(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	cm := &capMailer{}
	s.SetMailer(cm)
	if err := s.RequestPasswordReset(context.Background(), "nobody@example.com", "1.1.1.1"); err != nil {
		t.Fatalf("forgot must succeed silently for unknown email, got %v", err)
	}
	if cm.resetLink != "" {
		t.Fatal("must not send mail for unknown email")
	}
}
