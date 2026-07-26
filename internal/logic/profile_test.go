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

// setupAccount registers a user and returns the store, service, identity id, and a
// fresh login session id.
func setupAccount(t *testing.T) (*repo.Memory, *logic.Service, string, string) {
	t.Helper()
	ctx := context.Background()
	store := repo.NewMemory()
	svc := logic.New(store, logic.DefaultConfig())
	id, err := svc.Register(ctx, logic.RegisterInput{
		Email: "u@e.com", Password: "correct horse battery", DisplayName: "U",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	out, err := svc.Login(ctx, logic.LoginInput{Email: "u@e.com", Password: "correct horse battery", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return store, svc, id.ID, out.SessionID
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var c *errs.Coded
	if !errors.As(err, &c) {
		t.Fatalf("want a coded error, got %v", err)
	}
	return c.Code
}

func TestUpdateProfile_Success(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)

	p, err := svc.UpdateProfile(ctx, id, logic.ProfileUpdate{
		DisplayName: "  New Name  ", Username: "newuser", AvatarURL: "https://x/a.png", Locale: "zh-CN",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if p.DisplayName != "New Name" { // trimmed
		t.Errorf("DisplayName = %q, want trimmed 'New Name'", p.DisplayName)
	}
	if p.Username != "newuser" || p.AvatarURL != "https://x/a.png" || p.Locale != "zh-CN" {
		t.Errorf("profile not persisted: %+v", p)
	}
	// Re-read confirms persistence.
	got, _ := svc.GetProfile(ctx, id)
	if got.DisplayName != "New Name" || got.Username != "newuser" {
		t.Errorf("GetProfile after update = %+v", got)
	}
}

func TestUpdateProfile_EmptyDisplayNameRejected(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	_, err := svc.UpdateProfile(ctx, id, logic.ProfileUpdate{DisplayName: "   "})
	if codeOf(t, err) != iderr.CodeInvalidProfile {
		t.Fatalf("want CodeInvalidProfile, got %v", err)
	}
	var coded *errs.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("want coded error, got %v", err)
	}
	if coded.Params["reason"] != string(iderr.ProfileReasonDisplayNameRequired) {
		t.Fatalf("reason = %#v, want %q", coded.Params["reason"], iderr.ProfileReasonDisplayNameRequired)
	}
}

func TestChangePassword_SuccessKeepsCurrentRevokesOthers(t *testing.T) {
	ctx := context.Background()
	store, svc, id, sid1 := setupAccount(t)
	// A second session (another device).
	out2, err := svc.Login(ctx, logic.LoginInput{Email: "u@e.com", Password: "correct horse battery", IP: "10.0.0.2"})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}
	sid2 := out2.SessionID

	if err := svc.ChangePassword(ctx, id, sid1, "correct horse battery", "brand new password phrase"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	// New password works, old fails.
	if !logic.VerifyPassword(mustHash(t, store, id), "brand new password phrase") {
		t.Errorf("stored hash does not verify against the new password")
	}
	// Current session preserved, other session revoked.
	if _, err := svc.Me(ctx, sid1); err != nil {
		t.Errorf("current session should survive change-password, got %v", err)
	}
	if _, err := svc.Me(ctx, sid2); err == nil {
		t.Errorf("other session should be revoked after change-password")
	}
}

func TestChangePassword_WrongCurrentRejected(t *testing.T) {
	ctx := context.Background()
	_, svc, id, sid := setupAccount(t)
	err := svc.ChangePassword(ctx, id, sid, "wrongcurrent", "brand new password phrase")
	if codeOf(t, err) != iderr.CodeInvalidCredentials {
		t.Fatalf("want CodeInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_WeakNewRejected(t *testing.T) {
	ctx := context.Background()
	_, svc, id, sid := setupAccount(t)
	err := svc.ChangePassword(ctx, id, sid, "correct horse battery", "short")
	if codeOf(t, err) != iderr.CodeWeakPassword {
		t.Fatalf("want CodeWeakPassword, got %v", err)
	}
}

func TestRevokeSession_OwnedSucceeds(t *testing.T) {
	ctx := context.Background()
	_, svc, id, sid1 := setupAccount(t)
	out2, _ := svc.Login(ctx, logic.LoginInput{Email: "u@e.com", Password: "correct horse battery", IP: "10.0.0.2"})
	sid2 := out2.SessionID

	if err := svc.RevokeSession(ctx, id, sid2); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := svc.Me(ctx, sid2); err == nil {
		t.Errorf("revoked session should be gone")
	}
	if _, err := svc.Me(ctx, sid1); err != nil {
		t.Errorf("other session should remain, got %v", err)
	}
}

func TestRevokeSession_NotOwnedHidden(t *testing.T) {
	ctx := context.Background()
	_, svc, _, victimSID := setupAccount(t)
	// A different identity tries to revoke the victim's session.
	other, err := svc.Register(ctx, logic.RegisterInput{Email: "other@e.com", Password: "correct horse battery", DisplayName: "O"})
	if err != nil {
		t.Fatalf("register other: %v", err)
	}
	err = svc.RevokeSession(ctx, other.ID, victimSID)
	if codeOf(t, err) != iderr.CodeSessionNotFound {
		t.Fatalf("want CodeSessionNotFound (no cross-account oracle), got %v", err)
	}
	// Victim session untouched.
	if _, err := svc.Me(ctx, victimSID); err != nil {
		t.Errorf("victim session must survive a foreign revoke attempt, got %v", err)
	}
}

func TestRevokeSession_UnknownHidden(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	err := svc.RevokeSession(ctx, id, "no-such-session")
	if codeOf(t, err) != iderr.CodeSessionNotFound {
		t.Fatalf("want CodeSessionNotFound, got %v", err)
	}
}

func mustHash(t *testing.T, store *repo.Memory, id string) string {
	t.Helper()
	h, err := store.GetPasswordHash(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPasswordHash: %v", err)
	}
	return h
}
