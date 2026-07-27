package logic_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

func newSvc() *logic.Service { return logic.New(repo.NewMemory(), logic.DefaultConfig()) }

// codeOfErr returns the public Problem code, or "" for an unmapped error.
func codeOfErr(err error) string {
	if value, ok := iderr.Resolve(err); ok {
		return value.Code
	}
	return ""
}

func TestRegisterSuccess(t *testing.T) {
	svc := newSvc()
	id, err := svc.Register(context.Background(), logic.RegisterInput{
		Email: " New@User.com ", Password: "correct horse battery", DisplayName: "New",
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
	in := logic.RegisterInput{Email: "a@b.com", Password: "correct horse battery"}
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
	_, err := svc.Register(context.Background(), logic.RegisterInput{Email: "bad", Password: "correct horse battery"})
	if codeOfErr(err) != iderr.CodeInvalidEmail {
		t.Fatalf("want invalid_email, got %v", err)
	}
}
