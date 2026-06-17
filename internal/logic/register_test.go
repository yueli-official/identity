package logic_test

import (
	"context"
	"errors"
	"testing"

	"platform/gokit/errs"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

func newSvc() *logic.Service { return logic.New(repo.NewMemory(), logic.DefaultConfig()) }

// codeOfErr returns the *errs.Coded code, or "" if not a Coded error.
func codeOfErr(err error) string {
	var c *errs.Coded
	if errors.As(err, &c) {
		return c.Code
	}
	return ""
}

func TestRegisterSuccess(t *testing.T) {
	svc := newSvc()
	id, err := svc.Register(context.Background(), logic.RegisterInput{
		Email: " New@User.com ", Password: "longenough123", DisplayName: "New",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id.Email != "new@user.com" {
		t.Errorf("email not canonicalized: %q", id.Email)
	}
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	svc := newSvc()
	in := logic.RegisterInput{Email: "a@b.com", Password: "longenough123"}
	if _, err := svc.Register(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Register(context.Background(), in)
	if got := codeOfErr(err); got != iderr.CodeEmailTaken {
		t.Fatalf("want email_taken, got %q", got)
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	svc := newSvc()
	_, err := svc.Register(context.Background(), logic.RegisterInput{Email: "a@b.com", Password: "short"})
	if codeOfErr(err) != iderr.CodeWeakPassword {
		t.Fatalf("want weak_password, got %v", err)
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	svc := newSvc()
	_, err := svc.Register(context.Background(), logic.RegisterInput{Email: "bad", Password: "longenough123"})
	if codeOfErr(err) != iderr.CodeInvalidEmail {
		t.Fatalf("want invalid_email, got %v", err)
	}
}
