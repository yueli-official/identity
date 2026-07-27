package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/repo"
)

func TestMemory_PAT_InsertIncreasesID(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	id1, err := m.InsertPAT(ctx, repo.PATRow{
		IdentityID:  "u1",
		Name:        "tok-a",
		TokenHash:   "hash-a",
		TokenPrefix: "pre-a",
		Scopes:      []string{"read"},
	})
	if err != nil {
		t.Fatalf("InsertPAT 1: %v", err)
	}
	id2, err := m.InsertPAT(ctx, repo.PATRow{
		IdentityID:  "u1",
		Name:        "tok-b",
		TokenHash:   "hash-b",
		TokenPrefix: "pre-b",
		Scopes:      []string{"write"},
	})
	if err != nil {
		t.Fatalf("InsertPAT 2: %v", err)
	}
	if id2 <= id1 {
		t.Fatalf("expected id2 > id1; got id1=%d id2=%d", id1, id2)
	}
}

func TestMemory_PAT_InsertAutoFillsCreatedAt(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	before := time.Now()
	_, err := m.InsertPAT(ctx, repo.PATRow{
		IdentityID: "u1",
		Name:       "tok",
		TokenHash:  "hash-c",
		Scopes:     []string{},
		// CreatedAt intentionally zero
	})
	after := time.Now()
	if err != nil {
		t.Fatalf("InsertPAT: %v", err)
	}

	rows, err := m.ListPATByIdentity(ctx, "u1")
	if err != nil {
		t.Fatalf("ListPATByIdentity: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].CreatedAt.IsZero() {
		t.Fatal("CreatedAt must be auto-filled when zero")
	}
	if rows[0].CreatedAt.Before(before) || rows[0].CreatedAt.After(after) {
		t.Fatalf("CreatedAt %v not in [%v, %v]", rows[0].CreatedAt, before, after)
	}
}

func TestMemory_PAT_ListByIdentity_FilterAndOrder(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	// Insert for u1 (two tokens) and u2 (one token).
	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", Name: "first", TokenHash: "h1", Scopes: []string{"read"}})
	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u2", Name: "other", TokenHash: "h2", Scopes: []string{"write"}})
	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", Name: "second", TokenHash: "h3", Scopes: []string{"admin"}})

	rows, err := m.ListPATByIdentity(ctx, "u1")
	if err != nil {
		t.Fatalf("ListPATByIdentity u1: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows for u1, got %d", len(rows))
	}
	// Newest-first: "second" was inserted last, so it should be rows[0].
	if rows[0].Name != "second" {
		t.Fatalf("want rows[0].Name=second (newest), got %q", rows[0].Name)
	}
	if rows[1].Name != "first" {
		t.Fatalf("want rows[1].Name=first (oldest), got %q", rows[1].Name)
	}

	// u2 list must only return u2's row.
	u2rows, err := m.ListPATByIdentity(ctx, "u2")
	if err != nil {
		t.Fatalf("ListPATByIdentity u2: %v", err)
	}
	if len(u2rows) != 1 {
		t.Fatalf("want 1 row for u2, got %d", len(u2rows))
	}
}

func TestMemory_PAT_GetPATByHash_HitAndMiss(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	_, _ = m.InsertPAT(ctx, repo.PATRow{
		IdentityID:  "u1",
		Name:        "tok",
		TokenHash:   "hash-target",
		TokenPrefix: "pfx",
		Scopes:      []string{"read", "write"},
	})

	// Hit.
	row, found, err := m.GetPATByHash(ctx, "hash-target")
	if err != nil {
		t.Fatalf("GetPATByHash hit: %v", err)
	}
	if !found {
		t.Fatal("GetPATByHash: expected found=true")
	}
	if row.TokenHash != "hash-target" {
		t.Fatalf("GetPATByHash: wrong TokenHash %q", row.TokenHash)
	}
	if row.IdentityID != "u1" {
		t.Fatalf("GetPATByHash: wrong IdentityID %q", row.IdentityID)
	}

	// Miss.
	_, found2, err2 := m.GetPATByHash(ctx, "no-such-hash")
	if err2 != nil {
		t.Fatalf("GetPATByHash miss: %v", err2)
	}
	if found2 {
		t.Fatal("GetPATByHash: expected found=false for missing hash")
	}
}

func TestMemory_PAT_DeletePAT_OwnerMatch(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	id1, _ := m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", Name: "tok", TokenHash: "hd1", Scopes: []string{}})

	// Wrong owner — must NOT delete.
	deleted, err := m.DeletePAT(ctx, id1, "u2")
	if err != nil {
		t.Fatalf("DeletePAT wrong owner: %v", err)
	}
	if deleted {
		t.Fatal("DeletePAT with wrong owner must return false")
	}
	// Token must still exist.
	_, found, _ := m.GetPATByHash(ctx, "hd1")
	if !found {
		t.Fatal("token must still exist after wrong-owner delete attempt")
	}

	// Correct owner — must delete.
	deleted2, err := m.DeletePAT(ctx, id1, "u1")
	if err != nil {
		t.Fatalf("DeletePAT correct owner: %v", err)
	}
	if !deleted2 {
		t.Fatal("DeletePAT with correct owner must return true")
	}
	// Token must be gone.
	_, found2, _ := m.GetPATByHash(ctx, "hd1")
	if found2 {
		t.Fatal("token must be removed after correct-owner delete")
	}
}

func TestMemory_PAT_DeletePAT_MissingID(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	deleted, err := m.DeletePAT(ctx, 9999, "u1")
	if err != nil {
		t.Fatalf("DeletePAT missing: %v", err)
	}
	if deleted {
		t.Fatal("DeletePAT on missing id must return false")
	}
}

func TestMemory_PAT_TouchPATLastUsed(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	id, _ := m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", Name: "tok", TokenHash: "ht1", Scopes: []string{}})

	ts := time.Now().Add(-time.Minute).Truncate(time.Second)
	if err := m.TouchPATLastUsed(ctx, id, ts); err != nil {
		t.Fatalf("TouchPATLastUsed: %v", err)
	}

	rows, _ := m.ListPATByIdentity(ctx, "u1")
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].LastUsedAt == nil {
		t.Fatal("LastUsedAt must be set after Touch")
	}
	if !rows[0].LastUsedAt.Equal(ts) {
		t.Fatalf("LastUsedAt: want %v, got %v", ts, *rows[0].LastUsedAt)
	}

	// Touch on missing id must be a no-op (nil error).
	if err := m.TouchPATLastUsed(ctx, 9999, ts); err != nil {
		t.Fatalf("TouchPATLastUsed on missing id must return nil, got %v", err)
	}
}

func TestMemory_PAT_CountPATByIdentity(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	if n, err := m.CountPATByIdentity(ctx, "u1"); err != nil || n != 0 {
		t.Fatalf("initial count: want 0 nil, got %d %v", n, err)
	}

	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", TokenHash: "hc1", Scopes: []string{}})
	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u1", TokenHash: "hc2", Scopes: []string{}})
	_, _ = m.InsertPAT(ctx, repo.PATRow{IdentityID: "u2", TokenHash: "hc3", Scopes: []string{}})

	if n, err := m.CountPATByIdentity(ctx, "u1"); err != nil || n != 2 {
		t.Fatalf("u1 count: want 2 nil, got %d %v", n, err)
	}
	if n, err := m.CountPATByIdentity(ctx, "u2"); err != nil || n != 1 {
		t.Fatalf("u2 count: want 1 nil, got %d %v", n, err)
	}
}

func TestMemory_PAT_ScopesAliasing(t *testing.T) {
	ctx := context.Background()
	m := repo.NewMemory()

	originalScopes := []string{"read", "write"}
	id, err := m.InsertPAT(ctx, repo.PATRow{
		IdentityID: "u1",
		Name:       "alias-tok",
		TokenHash:  "hash-alias",
		Scopes:     originalScopes,
	})
	if err != nil {
		t.Fatalf("InsertPAT: %v", err)
	}

	// Mutate original slice after insert — stored state must not change.
	originalScopes[0] = "MUTATED"

	// Verify via ListPATByIdentity.
	rows, _ := m.ListPATByIdentity(ctx, "u1")
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Scopes[0] != "read" {
		t.Fatalf("InsertPAT: stored Scopes aliased to caller slice; got %v", rows[0].Scopes)
	}

	// Mutating the slice returned by List must not change stored state.
	rows[0].Scopes[0] = "LIST-MUTATED"
	rows2, _ := m.ListPATByIdentity(ctx, "u1")
	if rows2[0].Scopes[0] != "read" {
		t.Fatalf("ListPATByIdentity: returned Scopes aliased to internal state; got %v", rows2[0].Scopes)
	}

	// Verify via GetPATByHash.
	row, _, _ := m.GetPATByHash(ctx, "hash-alias")
	if row.Scopes[0] != "read" {
		t.Fatalf("GetPATByHash: Scopes aliased to internal state; got %v", row.Scopes)
	}

	// Mutating the Get result must not change stored state.
	row.Scopes[0] = "GET-MUTATED"
	row2, _, _ := m.GetPATByHash(ctx, "hash-alias")
	if row2.Scopes[0] != "read" {
		t.Fatalf("GetPATByHash: returned Scopes aliased to internal state after mutation; got %v", row2.Scopes)
	}

	// Touch to update LastUsedAt, then verify via List — Scopes still intact.
	_ = m.TouchPATLastUsed(ctx, id, time.Now())
	rows3, _ := m.ListPATByIdentity(ctx, "u1")
	if rows3[0].Scopes[0] != "read" {
		t.Fatalf("after Touch, Scopes corrupted; got %v", rows3[0].Scopes)
	}
}
