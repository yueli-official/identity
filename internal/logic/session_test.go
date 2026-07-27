package logic_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
)

func TestMeReturnsIdentityForValidSession(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "correct horse battery")
	out, _ := svc.Login(context.Background(), logic.LoginInput{Email: "a@b.com", Password: "correct horse battery"})
	id, err := svc.Me(context.Background(), out.SessionID)
	if err != nil || id.Email != "a@b.com" {
		t.Fatalf("me failed: %v %#v", err, id)
	}
}

func TestMeRejectsUnknownSession(t *testing.T) {
	svc := newSvc()
	_, err := svc.Me(context.Background(), "nope")
	if codeOfErr(err) != iderr.CodeNotAuthenticated {
		t.Fatalf("want not_authenticated, got %v", err)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "a@b.com", "correct horse battery")
	out, _ := svc.Login(context.Background(), logic.LoginInput{Email: "a@b.com", Password: "correct horse battery"})
	if err := svc.Logout(context.Background(), out.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Me(context.Background(), out.SessionID); codeOfErr(err) != iderr.CodeNotAuthenticated {
		t.Fatalf("session not cleared: %v", err)
	}
}
