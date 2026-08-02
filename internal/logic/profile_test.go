package logic_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
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

func TestUpdateSocialLinks_UsesDedicatedValidatedBoundary(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	links, err := svc.UpdateSocialLinks(ctx, id, []model.SocialLink{
		{Label: " GitHub ", URL: " https://github.com/yueli-official "},
	})
	if err != nil {
		t.Fatalf("UpdateSocialLinks: %v", err)
	}
	if len(links) != 1 || links[0].Label != "GitHub" || links[0].URL != "https://github.com/yueli-official" {
		t.Fatalf("normalized links = %#v", links)
	}
	profile, err := svc.GetProfile(ctx, id)
	if err != nil || len(profile.SocialLinks) != 1 {
		t.Fatalf("stored profile links = %#v, err=%v", profile.SocialLinks, err)
	}
	_, err = svc.UpdateSocialLinks(ctx, id, []model.SocialLink{{Label: "GitHub", URL: "javascript:alert(1)"}})
	if codeOf(t, err) != iderr.CodeInvalidProfile {
		t.Fatalf("invalid URL should be rejected: %v", err)
	}
	coded, _ := iderr.Resolve(err)
	if coded.Params["reason"] != string(iderr.ProfileReasonSocialLinksInvalid) {
		t.Fatalf("reason = %#v", coded.Params["reason"])
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	value, ok := iderr.Resolve(err)
	if !ok {
		t.Fatalf("want a public Problem error, got %v", err)
	}
	return value.Code
}

func TestUpdateProfile_Success(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	if err := svc.SetProfileImage(ctx, id, "avatar", "31Pj0mXv7cfR5fdZIUvra", "asset-avatar"); err != nil {
		t.Fatalf("SetProfileImage: %v", err)
	}

	p, err := svc.UpdateProfile(ctx, id, logic.ProfileUpdate{
		DisplayName: "  New Name  ", Handle: " New_User ", Bio: " hello ", Locale: "zh-CN",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if p.DisplayName != "New Name" { // trimmed
		t.Errorf("DisplayName = %q, want trimmed 'New Name'", p.DisplayName)
	}
	if p.Handle != "new_user" || p.AvatarMediaKey != "31Pj0mXv7cfR5fdZIUvra" || p.Locale != "zh-CN" {
		t.Errorf("profile not persisted: %+v", p)
	}
	if p.AvatarAssetID != "asset-avatar" {
		t.Errorf("generic update replaced Asset ownership: %+v", p)
	}
	// Re-read confirms persistence.
	got, _ := svc.GetProfile(ctx, id)
	if got.DisplayName != "New Name" || got.Handle != "new_user" {
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
	coded, ok := iderr.Resolve(err)
	if !ok {
		t.Fatalf("want public Problem error, got %v", err)
	}
	if coded.Params["reason"] != string(iderr.ProfileReasonDisplayNameRequired) {
		t.Fatalf("reason = %#v, want %q", coded.Params["reason"], iderr.ProfileReasonDisplayNameRequired)
	}
}

func TestUpdateProfile_InvalidHandleRejected(t *testing.T) {
	ctx := context.Background()
	_, svc, id, _ := setupAccount(t)
	_, err := svc.UpdateProfile(ctx, id, logic.ProfileUpdate{DisplayName: "Alice", Handle: "admin"})
	if codeOf(t, err) != iderr.CodeInvalidProfile {
		t.Fatalf("want CodeInvalidProfile, got %v", err)
	}
	coded, _ := iderr.Resolve(err)
	if coded.Params["reason"] != string(iderr.ProfileReasonHandleInvalid) {
		t.Fatalf("reason = %#v, want %q", coded.Params["reason"], iderr.ProfileReasonHandleInvalid)
	}
}

func TestUpdateProfile_HandleCannotBeClaimedTwice(t *testing.T) {
	ctx := context.Background()
	_, svc, firstID, _ := setupAccount(t)
	second, err := svc.Register(ctx, logic.RegisterInput{
		Email: "second@e.com", Password: "correct horse battery", DisplayName: "Second",
	})
	if err != nil {
		t.Fatalf("register second: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, firstID, logic.ProfileUpdate{DisplayName: "First", Handle: "alice"}); err != nil {
		t.Fatalf("claim first handle: %v", err)
	}
	_, err = svc.UpdateProfile(ctx, second.ID, logic.ProfileUpdate{DisplayName: "Second", Handle: "ALICE"})
	if codeOf(t, err) != iderr.CodeHandleUnavailable {
		t.Fatalf("want CodeHandleUnavailable, got %v", err)
	}
}

func TestPublicUserByHistoricalHandleResolvesCanonicalUser(t *testing.T) {
	ctx := context.Background()
	_, svc, identityID, _ := setupAccount(t)
	if _, err := svc.UpdateProfile(ctx, identityID, logic.ProfileUpdate{DisplayName: "Alice", Handle: "alice"}); err != nil {
		t.Fatalf("claim handle: %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, identityID, logic.ProfileUpdate{DisplayName: "Alice", Handle: "alice_new"}); err != nil {
		t.Fatalf("rename handle: %v", err)
	}
	resolved, err := svc.PublicUserByHandle(ctx, "ALICE")
	if err != nil {
		t.Fatalf("resolve historical handle: %v", err)
	}
	if resolved.Handle != "alice_new" {
		t.Fatalf("canonical handle = %q, want alice_new", resolved.Handle)
	}
}

func TestPublicUserHiddenAfterDeletion(t *testing.T) {
	ctx := context.Background()
	_, svc, targetID, _ := setupAccount(t)
	target, err := svc.GetByID(ctx, targetID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	admin, err := svc.Register(ctx, logic.RegisterInput{
		Email: "admin@e.com", Password: "correct horse battery", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	if _, err := svc.PublicUser(ctx, target.UserKey); err != nil {
		t.Fatalf("public user before deletion: %v", err)
	}
	if err := svc.AdminDeleteUser(actor.WithIdentity(ctx, admin.ID), targetID); err != nil {
		t.Fatalf("AdminDeleteUser: %v", err)
	}
	if _, err := svc.PublicUser(ctx, target.UserKey); codeOf(t, err) != iderr.CodeIdentityNotFound {
		t.Fatalf("deleted user remained public: %v", err)
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
