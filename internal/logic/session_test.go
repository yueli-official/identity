package logic_test

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
)

func TestReauthenticateRefreshesCurrentSessionWithoutCreatingAnother(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "reauth@example.com", "correct horse battery")
	out, err := svc.Login(context.Background(), logic.LoginInput{
		Email: "reauth@example.com", Password: "correct horse battery",
	})
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := svc.AuthenticatedSession(context.Background(), out.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)

	if err := svc.Reauthenticate(context.Background(), out.SessionID, "correct horse battery"); err != nil {
		t.Fatalf("Reauthenticate() error = %v", err)
	}
	after, identity, err := svc.AuthenticatedSession(context.Background(), out.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Authentication.EventID == before.Authentication.EventID {
		t.Fatalf("session was replaced or authentication event was not refreshed: before=%+v after=%+v", before, after)
	}
	if after.Authentication.AuthenticatedAt.Before(before.Authentication.AuthenticatedAt) ||
		len(after.Authentication.Methods) != 1 || after.Authentication.Methods[0] != authentication.MethodPassword {
		t.Fatalf("unexpected refreshed authentication: %+v", after.Authentication)
	}
	sessions, err := svc.ListSessions(context.Background(), identity.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions after reauthentication = %d, err=%v", len(sessions), err)
	}
}

func TestReauthenticateRejectsWrongPasswordWithoutRefreshingSession(t *testing.T) {
	svc := newSvc()
	seedUser(t, svc, "reauth-wrong@example.com", "correct horse battery")
	out, _ := svc.Login(context.Background(), logic.LoginInput{
		Email: "reauth-wrong@example.com", Password: "correct horse battery",
	})
	before, _, _ := svc.AuthenticatedSession(context.Background(), out.SessionID)

	err := svc.Reauthenticate(context.Background(), out.SessionID, "wrong password")
	if codeOfErr(err) != iderr.CodeInvalidCredentials {
		t.Fatalf("Reauthenticate() code = %v, want invalid_credentials", err)
	}
	after, _, _ := svc.AuthenticatedSession(context.Background(), out.SessionID)
	if after.Authentication.EventID != before.Authentication.EventID {
		t.Fatal("wrong password refreshed the session")
	}
}

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
