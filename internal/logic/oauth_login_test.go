package logic_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

func TestOAuthLogin_ImplicitRegister(t *testing.T) {
	s := newSvc()
	out, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-1", Email: "new@example.com",
		EmailVerified: true, DisplayName: "New",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.SessionID == "" || out.Identity.Email != "new@example.com" || !out.Identity.EmailVerified {
		t.Fatalf("bad implicit register: %+v", out)
	}
}

func TestOAuthLogin_ReturningUser(t *testing.T) {
	s := newSvc()
	in := logic.OAuthLoginInput{Provider: "google", ProviderUID: "sub-2", Email: "ret@example.com", EmailVerified: true}
	first, _ := s.OAuthLogin(context.Background(), in)
	second, err := s.OAuthLogin(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity.ID != second.Identity.ID {
		t.Fatal("returning user should resolve same identity")
	}
	if first.SessionID == second.SessionID {
		t.Fatal("each login should mint a fresh session")
	}
}

func TestOAuthLogin_LinkByVerifiedEmail(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	base, _ := m.CreateIdentityWithProfile(context.Background(), repo.NewIdentityInput{Email: "link@example.com", DisplayName: "L", PasswordHash: "h"})
	out, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-3", Email: "link@example.com", EmailVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Identity.ID != base.ID {
		t.Fatal("verified email should link to existing identity")
	}
}

func TestOAuthLogin_UnverifiedEmailCollision_Rejected(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	_, _ = m.CreateIdentityWithProfile(context.Background(), repo.NewIdentityInput{Email: "taken@example.com", DisplayName: "T", PasswordHash: "h"})
	_, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-4", Email: "taken@example.com", EmailVerified: false,
	})
	if err == nil {
		t.Fatal("unverified-email collision must be rejected (§10)")
	}
	if codeOfErr(err) != iderr.CodeOAuthEmailConflict {
		t.Fatalf("want oauth_email_conflict, got %v", err)
	}
}

func TestOAuthLogin_NoEmail_Rejected(t *testing.T) {
	s := newSvc()
	_, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-5", Email: "", EmailVerified: false,
	})
	if codeOfErr(err) != iderr.CodeOAuthNoEmail {
		t.Fatalf("want oauth_no_email, got %v", err)
	}
}
