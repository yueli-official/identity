package repo_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

func TestMemoryIdentityCreateAndGet(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	id, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "a@b.com", DisplayName: "A", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.GetByEmail(ctx, "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id.ID {
		t.Fatalf("id mismatch")
	}
	if _, err := m.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "a@b.com", PasswordHash: "h"}); err != repo.ErrEmailTaken {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestMemorySessionLifecycle(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()
	s := model.Session{ID: "s1", IdentityID: "u1"}
	if err := m.CreateSession(ctx, s, 0); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetSession(ctx, "s1")
	if err != nil || got.IdentityID != "u1" {
		t.Fatalf("get session: %v %#v", err, got)
	}
	list, _ := m.ListSessionsByIdentity(ctx, "u1")
	if len(list) != 1 {
		t.Fatalf("want 1 session, got %d", len(list))
	}
	_ = m.DeleteSession(ctx, "s1")
	if _, err := m.GetSession(ctx, "s1"); err != repo.ErrSessionNotFound {
		t.Fatalf("want ErrSessionNotFound, got %v", err)
	}
}
