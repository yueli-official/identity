package logic

import (
	"context"
	"testing"

	"platform/services/identity/internal/repo"
)

type fakeRevoker struct{ bySession, byIdentity []string }

func (f *fakeRevoker) RevokeRefreshBySession(_ context.Context, s string) error {
	f.bySession = append(f.bySession, s)
	return nil
}
func (f *fakeRevoker) RevokeRefreshByIdentity(_ context.Context, s string) error {
	f.byIdentity = append(f.byIdentity, s)
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

	if err := svc.LogoutAll(ctx, "id-1"); err != nil {
		t.Fatal(err)
	}
	if len(fr.byIdentity) != 1 || fr.byIdentity[0] != "id-1" {
		t.Fatalf("expected RevokeRefreshByIdentity(id-1), got %v", fr.byIdentity)
	}
}
