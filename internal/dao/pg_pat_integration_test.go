//go:build integration

package dao_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yueli-official/identity/internal/dao"
	"github.com/yueli-official/identity/internal/repo"
)

// TestPGPATRoundTrip exercises the full PATRepo lifecycle against a real
// PostgreSQL database.
//
// Requires TEST_PG_LINK env var and migrations 0001–0008 applied.
// Run with: go test -tags=integration ./internal/dao/ -run TestPGPAT
func TestPGPATRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	// We need a real identity row because pat_tokens.identity_id is a FK.
	// Reuse the helper from pg_integration_test.go (same package).
	email := "pat-test-" + uuid.NewString() + "@example.com"
	identity, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: email, DisplayName: "PAT Tester", PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("CreateIdentityWithProfile: %v", err)
	}
	t.Cleanup(func() {
		// pat_tokens rows are cascade-deleted when the identity is removed.
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	identityID := identity.ID

	// ── Insert ────────────────────────────────────────────────────────────────
	hash1 := "hash-" + uuid.NewString()
	hash2 := "hash-" + uuid.NewString()

	id1, err := d.InsertPAT(ctx, repo.PATRow{
		IdentityID:  identityID,
		Name:        "token-one",
		TokenHash:   hash1,
		TokenPrefix: "tok_one",
		Scopes:      []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("InsertPAT(1): %v", err)
	}
	if id1 == 0 {
		t.Fatal("InsertPAT returned zero id")
	}

	id2, err := d.InsertPAT(ctx, repo.PATRow{
		IdentityID:  identityID,
		Name:        "token-two",
		TokenHash:   hash2,
		TokenPrefix: "tok_two",
		Scopes:      []string{"read"},
	})
	if err != nil {
		t.Fatalf("InsertPAT(2): %v", err)
	}

	// ── GetPATByHash — hit ────────────────────────────────────────────────────
	got, found, err := d.GetPATByHash(ctx, hash1)
	if err != nil {
		t.Fatalf("GetPATByHash: %v", err)
	}
	if !found {
		t.Fatal("GetPATByHash: want found=true, got false")
	}
	if got.ID != id1 {
		t.Errorf("GetPATByHash: want id=%d, got %d", id1, got.ID)
	}
	if got.Name != "token-one" {
		t.Errorf("GetPATByHash: want name=%q, got %q", "token-one", got.Name)
	}
	if len(got.Scopes) != 2 || got.Scopes[0] != "read" || got.Scopes[1] != "write" {
		t.Errorf("GetPATByHash: want scopes=[read write], got %v", got.Scopes)
	}
	if got.ExpiresAt != nil {
		t.Errorf("GetPATByHash: want ExpiresAt=nil, got %v", got.ExpiresAt)
	}
	if got.LastUsedAt != nil {
		t.Errorf("GetPATByHash: want LastUsedAt=nil, got %v", got.LastUsedAt)
	}
	if got.CreatedAt.IsZero() {
		t.Error("GetPATByHash: want non-zero CreatedAt")
	}

	// ── GetPATByHash — miss ───────────────────────────────────────────────────
	_, found, err = d.GetPATByHash(ctx, "no-such-hash")
	if err != nil {
		t.Fatalf("GetPATByHash(miss): %v", err)
	}
	if found {
		t.Error("GetPATByHash(miss): want found=false, got true")
	}

	// ── ListPATByIdentity — DESC order (id2 > id1) ───────────────────────────
	list, err := d.ListPATByIdentity(ctx, identityID)
	if err != nil {
		t.Fatalf("ListPATByIdentity: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListPATByIdentity: want 2, got %d", len(list))
	}
	// Newest-first: id2 was inserted second so should appear first.
	if list[0].ID != id2 {
		t.Errorf("ListPATByIdentity: want first id=%d (newest), got %d", id2, list[0].ID)
	}
	if list[1].ID != id1 {
		t.Errorf("ListPATByIdentity: want second id=%d (older), got %d", id1, list[1].ID)
	}

	// ── CountPATByIdentity ────────────────────────────────────────────────────
	n, err := d.CountPATByIdentity(ctx, identityID)
	if err != nil {
		t.Fatalf("CountPATByIdentity: %v", err)
	}
	if n != 2 {
		t.Errorf("CountPATByIdentity: want 2, got %d", n)
	}

	// ── TouchPATLastUsed ──────────────────────────────────────────────────────
	touchTime := time.Now().UTC().Truncate(time.Second)
	if err := d.TouchPATLastUsed(ctx, id1, touchTime); err != nil {
		t.Fatalf("TouchPATLastUsed: %v", err)
	}
	after, found, err := d.GetPATByHash(ctx, hash1)
	if err != nil || !found {
		t.Fatalf("GetPATByHash after touch: err=%v found=%v", err, found)
	}
	if after.LastUsedAt == nil {
		t.Fatal("TouchPATLastUsed: LastUsedAt is still nil after touch")
	}
	if !after.LastUsedAt.Equal(touchTime) {
		t.Errorf("TouchPATLastUsed: want %v, got %v", touchTime, *after.LastUsedAt)
	}

	// ── DeletePAT — wrong owner does NOT delete ───────────────────────────────
	otherID := uuid.NewString() // fake identity, not in DB
	deleted, err := d.DeletePAT(ctx, id1, otherID)
	if err != nil {
		t.Fatalf("DeletePAT(wrong owner): %v", err)
	}
	if deleted {
		t.Error("DeletePAT(wrong owner): want deleted=false, got true")
	}
	// Verify it still exists.
	_, found, err = d.GetPATByHash(ctx, hash1)
	if err != nil || !found {
		t.Fatalf("after wrong-owner delete, token should still exist: err=%v found=%v", err, found)
	}

	// ── DeletePAT — correct owner deletes ────────────────────────────────────
	deleted, err = d.DeletePAT(ctx, id1, identityID)
	if err != nil {
		t.Fatalf("DeletePAT(correct owner): %v", err)
	}
	if !deleted {
		t.Error("DeletePAT(correct owner): want deleted=true, got false")
	}
	// Verify it no longer exists.
	_, found, err = d.GetPATByHash(ctx, hash1)
	if err != nil {
		t.Fatalf("GetPATByHash after delete: %v", err)
	}
	if found {
		t.Error("GetPATByHash after delete: want found=false, got true")
	}

	// Count is now 1.
	n, err = d.CountPATByIdentity(ctx, identityID)
	if err != nil {
		t.Fatalf("CountPATByIdentity after delete: %v", err)
	}
	if n != 1 {
		t.Errorf("CountPATByIdentity after delete: want 1, got %d", n)
	}
}

// TestPGPATNilScopesRoundTrip verifies that inserting a PAT with nil Scopes
// stores "[]" in Postgres and reads back as []string{} (not nil).
func TestPGPATNilScopesRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	email := "pat-nilscopes-" + uuid.NewString() + "@example.com"
	identity, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: email, DisplayName: "Nil Scopes Test", PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("CreateIdentityWithProfile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	hash := "hash-nilscopes-" + uuid.NewString()
	_, err = d.InsertPAT(ctx, repo.PATRow{
		IdentityID:  identity.ID,
		Name:        "nil-scopes-token",
		TokenHash:   hash,
		TokenPrefix: "tok_nil",
		Scopes:      nil, // explicit nil
	})
	if err != nil {
		t.Fatalf("InsertPAT(nil scopes): %v", err)
	}

	got, found, err := d.GetPATByHash(ctx, hash)
	if err != nil || !found {
		t.Fatalf("GetPATByHash(nil scopes): err=%v found=%v", err, found)
	}
	if got.Scopes == nil {
		t.Error("Scopes: want []string{} (non-nil), got nil")
	}
	if len(got.Scopes) != 0 {
		t.Errorf("Scopes: want empty slice, got %v", got.Scopes)
	}
}

// TestPGPATNullExpiresAt verifies that a PAT with nil ExpiresAt stores NULL
// in Postgres and reads back as nil *time.Time.
func TestPGPATNullExpiresAt(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	email := "pat-nullexp-" + uuid.NewString() + "@example.com"
	identity, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: email, DisplayName: "Null ExpiresAt Test", PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("CreateIdentityWithProfile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	// Insert with nil ExpiresAt.
	hashNoExp := "hash-noexp-" + uuid.NewString()
	_, err = d.InsertPAT(ctx, repo.PATRow{
		IdentityID:  identity.ID,
		Name:        "no-expiry",
		TokenHash:   hashNoExp,
		TokenPrefix: "tok_ne",
		Scopes:      []string{},
		ExpiresAt:   nil,
	})
	if err != nil {
		t.Fatalf("InsertPAT(nil ExpiresAt): %v", err)
	}

	got, found, err := d.GetPATByHash(ctx, hashNoExp)
	if err != nil || !found {
		t.Fatalf("GetPATByHash(nil ExpiresAt): err=%v found=%v", err, found)
	}
	if got.ExpiresAt != nil {
		t.Errorf("ExpiresAt: want nil (NULL roundtrip), got %v", got.ExpiresAt)
	}

	// Insert with non-nil ExpiresAt and verify it round-trips.
	hashWithExp := "hash-withexp-" + uuid.NewString()
	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	_, err = d.InsertPAT(ctx, repo.PATRow{
		IdentityID:  identity.ID,
		Name:        "with-expiry",
		TokenHash:   hashWithExp,
		TokenPrefix: "tok_we",
		Scopes:      []string{},
		ExpiresAt:   &exp,
	})
	if err != nil {
		t.Fatalf("InsertPAT(non-nil ExpiresAt): %v", err)
	}

	got2, found, err := d.GetPATByHash(ctx, hashWithExp)
	if err != nil || !found {
		t.Fatalf("GetPATByHash(non-nil ExpiresAt): err=%v found=%v", err, found)
	}
	if got2.ExpiresAt == nil {
		t.Fatal("ExpiresAt: want non-nil, got nil")
	}
	if !got2.ExpiresAt.Equal(exp) {
		t.Errorf("ExpiresAt: want %v, got %v", exp, *got2.ExpiresAt)
	}
}
