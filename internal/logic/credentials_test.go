package logic_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
)

func TestListCredentials_PasswordOnly(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	cs, err := svc.ListCredentials(ctx, id)
	if err != nil {
		t.Fatalf("ListCredentials: %v", err)
	}
	if !cs.HasPassword {
		t.Errorf("HasPassword = false, want true (password account)")
	}
	if len(cs.OAuth) != 0 {
		t.Errorf("OAuth = %v, want empty", cs.OAuth)
	}
}

func TestBindOAuth_SuccessAndIdempotent(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)

	if err := svc.BindOAuth(ctx, id, "google", "uid-a", "a@e.com", true); err != nil {
		t.Fatalf("BindOAuth: %v", err)
	}
	// Idempotent: binding the same provider+uid to the same identity again is ok.
	if err := svc.BindOAuth(ctx, id, "google", "uid-a", "a@e.com", true); err != nil {
		t.Fatalf("BindOAuth (repeat): %v", err)
	}
	cs, _ := svc.ListCredentials(ctx, id)
	if len(cs.OAuth) != 1 || cs.OAuth[0].Provider != "google" {
		t.Fatalf("OAuth = %v, want [google]", cs.OAuth)
	}
}

func TestBindOAuth_ConflictWithAnotherIdentity(t *testing.T) {
	ctx := context.Background()
	_, svc, idA, _ := setupAccount(t)

	// Identity B owns google uid "shared" via an oauth-only implicit register.
	if _, err := svc.OAuthLogin(ctx, logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "shared", Email: "b@e.com", EmailVerified: true, DisplayName: "B",
	}); err != nil {
		t.Fatalf("seed B via OAuthLogin: %v", err)
	}
	// A tries to bind the same google account → conflict.
	err := svc.BindOAuth(ctx, idA, "google", "shared", "b@e.com", true)
	if codeOf(t, err) != iderr.CodeCredentialConflict {
		t.Fatalf("want CodeCredentialConflict, got %v", err)
	}
}

func TestUnbindCredential_Success(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	if err := svc.BindOAuth(ctx, id, "google", "uid-a", "a@e.com", true); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := svc.UnbindCredential(ctx, id, "google"); err != nil {
		t.Fatalf("UnbindCredential: %v", err)
	}
	cs, _ := svc.ListCredentials(ctx, id)
	if len(cs.OAuth) != 0 {
		t.Errorf("OAuth = %v, want empty after unbind", cs.OAuth)
	}
	if !cs.HasPassword {
		t.Errorf("password credential should remain")
	}
}

func TestUnbindCredential_LastRejected(t *testing.T) {
	ctx := context.Background()
	_, svc, _, _ := setupAccount(t)
	// An oauth-ONLY identity (no password): unbinding its only google credential
	// must be refused.
	out, err := svc.OAuthLogin(ctx, logic.OAuthLoginInput{
		Provider: "google", ProviderUID: "solo", Email: "solo@e.com", EmailVerified: true, DisplayName: "Solo",
	})
	if err != nil {
		t.Fatalf("oauth-only register: %v", err)
	}
	err = svc.UnbindCredential(ctx, out.Identity.ID, "google")
	if codeOf(t, err) != iderr.CodeLastCredential {
		t.Fatalf("want CodeLastCredential, got %v", err)
	}
}

func TestUnbindCredential_NotBound(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	// Bind google so there are 2 credentials (so the last-credential guard passes),
	// then try to unbind a provider that isn't bound.
	if err := svc.BindOAuth(ctx, id, "google", "uid-a", "a@e.com", true); err != nil {
		t.Fatalf("bind: %v", err)
	}
	err := svc.UnbindCredential(ctx, id, "github")
	if codeOf(t, err) != iderr.CodeCredentialNotFound {
		t.Fatalf("want CodeCredentialNotFound, got %v", err)
	}
}
