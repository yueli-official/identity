package logic_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
)

func seedUser(t *testing.T, svc *logic.Service, email, pw string) {
	t.Helper()
	if _, err := svc.Register(context.Background(), logic.RegisterInput{Email: email, Password: pw}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginSuccessCreatesSession(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "longenough123")
	out, err := svc.Login(context.Background(), logic.LoginInput{Email: "A@B.com", Password: "longenough123", IP: "1.1.1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.SessionID == "" {
		t.Error("no session id returned")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "longenough123")
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
	seedUser(t, svc, "a@b.com", "longenough123")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(ctx, logic.LoginInput{Email: "a@b.com", Password: "nope", IP: "1.1.1.1"})
	}
	// 6th attempt (even with correct password) is locked out.
	_, err := svc.Login(ctx, logic.LoginInput{Email: "a@b.com", Password: "longenough123", IP: "1.1.1.1"})
	if codeOfErr(err) != iderr.CodeAccountLocked {
		t.Fatalf("want account_locked, got %v", err)
	}
}
