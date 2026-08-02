package logic

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/repo"
)

type fakeRevoker struct{ bySession, bySubject []string }

func (f *fakeRevoker) RevokeRefreshBySession(_ context.Context, s string) error {
	f.bySession = append(f.bySession, s)
	return nil
}
func (f *fakeRevoker) RevokeRefreshBySubject(_ context.Context, s string) error {
	f.bySubject = append(f.bySubject, s)
	return nil
}

func TestLogoutRevokesSessionBoundRefresh(t *testing.T) {
	ctx := context.Background()
	store := repo.NewMemory()
	svc := New(store, DefaultConfig())
	fr := &fakeRevoker{}
	svc.SetRefreshRevoker(fr)

	if err := svc.Logout(ctx, "sess-1"); err != nil {
		t.Fatal(err)
	}
	if len(fr.bySession) != 1 || fr.bySession[0] != "sess-1" {
		t.Fatalf("expected RevokeRefreshBySession(sess-1), got %v", fr.bySession)
	}

	identity, err := svc.Register(ctx, RegisterInput{
		Email: "revoke@example.com", Password: "correct horse battery", DisplayName: "Revoke",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LogoutAll(ctx, identity.ID); err != nil {
		t.Fatal(err)
	}
	if len(fr.bySubject) != 1 || fr.bySubject[0] != identity.UserKey {
		t.Fatalf("expected RevokeRefreshBySubject(%s), got %v", identity.UserKey, fr.bySubject)
	}
}
