package logic_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/oauthlogin"
	"github.com/yueli-official/identity/internal/repo"
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

func TestOAuthLogin_DoesNotAutoLinkByVerifiedEmail(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	base, _ := m.CreateIdentityWithProfile(context.Background(), repo.NewIdentityInput{Email: "link@example.com", DisplayName: "L", PasswordHash: "h"})
	_, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-3", Email: "link@example.com", EmailVerified: true,
	})
	if codeOfErr(err) != iderr.CodeOAuthEmailConflict {
		t.Fatalf("verified provider email must not silently link an existing account: %v", err)
	}
	credentials, err := m.ListOAuthCredentials(context.Background(), base.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 0 {
		t.Fatalf("existing account gained a credential without step-up: %+v", credentials)
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
		t.Fatal("unverified-email collision must be rejected")
	}
	if codeOfErr(err) != iderr.CodeOAuthEmailUnverified {
		t.Fatalf("want oauth_email_unverified, got %v", err)
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

func TestOAuthLogin_UnverifiedNewEmail_Rejected(t *testing.T) {
	s := newSvc()
	_, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "sub-unverified", Email: "new@example.com", EmailVerified: false,
	})
	if codeOfErr(err) != iderr.CodeOAuthEmailUnverified {
		t.Fatalf("want oauth_email_unverified, got %v", err)
	}
}

func TestOAuthLogin_ExistingOnlyRequiresPriorBinding(t *testing.T) {
	s := newSvc()
	_, err := s.OAuthLogin(context.Background(), logic.OAuthLoginInput{
		Provider: "qq", ProviderUID: "openid-1",
		RegistrationPolicy: oauthlogin.RegistrationExistingOnly,
	})
	if codeOfErr(err) != iderr.CodeOAuthBindingRequired {
		t.Fatalf("want oauth_binding_required, got %v", err)
	}
}

func TestOAuthLogin_DoesNotLinkCredentialToDisabledIdentity(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	base, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "disabled@example.com", DisplayName: "Disabled", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetIdentityStatus(ctx, base.ID, model.StatusDisabled); err != nil {
		t.Fatal(err)
	}

	_, err = s.OAuthLogin(ctx, logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "disabled-provider-sub",
		Email: "disabled@example.com", EmailVerified: true,
	})
	if codeOfErr(err) != iderr.CodeOAuthEmailConflict {
		t.Fatalf("unbound provider must not disclose account status, got %v", err)
	}
	credentials, err := m.ListOAuthCredentials(ctx, base.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 0 {
		t.Fatalf("disabled identity gained credentials: %+v", credentials)
	}
}
