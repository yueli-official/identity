package logic_test

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

func seedUser(t *testing.T, svc *logic.Service, email, pw string) {
	t.Helper()
	if _, err := svc.Register(context.Background(), logic.RegisterInput{Email: email, Password: pw}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginSuccessCreatesSession(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "correct horse battery")
	out, err := svc.Login(context.Background(), logic.LoginInput{Email: "A@B.com", Password: "correct horse battery", IP: "1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.SessionID == "" {
		t.Error("no session id returned")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "correct horse battery")
	_, err := svc.Login(context.Background(), logic.LoginInput{Email: "a@b.com", Password: "nope", IP: "1.1.1.1"})
	if codeOfErr(err) != iderr.CodeInvalidCredentials {
		t.Fatalf("want invalid_credentials, got %v", err)
	}
}

func TestLoginUnknownEmailIsGeneric(t *testing.T) {
	svc := newSvc()
	_, err := svc.Login(context.Background(), logic.LoginInput{Email: "ghost@b.com", Password: "x", IP: "1.1.1.1"})
	if codeOfErr(err) != iderr.CodeInvalidCredentials { // no account enumeration
		t.Fatalf("want invalid_credentials, got %v", err)
	}
}

func TestLoginLocksAfterMaxFailures(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "correct horse battery")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(ctx, logic.LoginInput{Email: "a@b.com", Password: "nope", IP: "1.1.1.1"})
	}
	// 6th attempt (even with correct password) is locked out.
	_, err := svc.Login(ctx, logic.LoginInput{Email: "a@b.com", Password: "correct horse battery", IP: "1.1.1.1"})
	if codeOfErr(err) != iderr.CodeAccountLocked {
		t.Fatalf("want account_locked, got %v", err)
	}
}

func TestLoginProgressivelyUpgradesLegacyBcrypt(t *testing.T) {
	ctx := context.Background()
	store := repo.NewMemory()
	legacy, err := bcrypt.GenerateFromPassword(
		[]byte("legacy password phrase"), bcrypt.MinCost,
	)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "legacy@example.test", PasswordHash: string(legacy),
	})
	if err != nil {
		t.Fatal(err)
	}
	service := logic.New(store, logic.DefaultConfig())
	if _, err := service.Login(ctx, logic.LoginInput{
		Email: identity.Email, Password: "legacy password phrase",
	}); err != nil {
		t.Fatal(err)
	}
	upgraded, err := store.GetPasswordHash(ctx, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(upgraded, "$argon2id$v=19$") {
		t.Fatalf("upgraded hash = %q", upgraded)
	}
	if !logic.VerifyPassword(upgraded, "legacy password phrase") {
		t.Fatal("upgraded hash did not verify")
	}
}
