//go:build integration

package oidc

import (
	"context"
	"os"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

// newTestDB returns a gdb.DB connected via TEST_PG_LINK (same pattern as
// internal/dao/pg_integration_test.go). Skips when the env var is unset.
func newTestDB(t *testing.T) gdb.DB {
	t.Helper()
	link := os.Getenv("TEST_PG_LINK") // e.g. pgsql:user:pass@tcp(127.0.0.1:5432)/identity_test
	if link == "" {
		t.Skip("TEST_PG_LINK not set; skipping pg integration test (requires migrations 0001-0003 applied)")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: link})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// cleanupGeneric removes a specific row to keep tests hermetic.
func cleanupGeneric(ctx context.Context, db gdb.DB, kind, sig string) {
	_, _ = db.Model("oidc_oauth_requests").Ctx(ctx).
		Where("kind", kind).Where("signature", sig).Delete()
}

// cleanupRefresh removes a specific refresh token row.
func cleanupRefresh(ctx context.Context, db gdb.DB, sig string) {
	_, _ = db.Model("oidc_refresh_tokens").Ctx(ctx).Where("signature", sig).Delete()
}

// TestPGBackendGenericRoundTrip exercises PutGeneric/GetGeneric/DeactivateGeneric/DeleteGeneric.
func TestPGBackendGenericRoundTrip(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	be := NewPGBackend(db)

	const kind, sig = "authcode", "pg-sig-1"
	t.Cleanup(func() { cleanupGeneric(ctx, db, kind, sig) })

	rec := Record{
		RequestID: "req-pg-1", ClientID: "c1", Subject: "sub-1",
		Active: true, ExpiresAt: time.Now().Add(time.Hour),
		Data: []byte(`{"pg":1}`),
	}
	if err := be.PutGeneric(ctx, kind, sig, rec); err != nil {
		t.Fatal(err)
	}

	got, err := be.GetGeneric(ctx, kind, sig)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != `{"pg":1}` || !got.Active {
		t.Fatalf("bad record: %+v", got)
	}

	// Upsert (overwrite) — same sig, new data.
	rec.Data = []byte(`{"pg":2}`)
	if err := be.PutGeneric(ctx, kind, sig, rec); err != nil {
		t.Fatal(err)
	}
	got2, err := be.GetGeneric(ctx, kind, sig)
	if err != nil {
		t.Fatal(err)
	}
	if string(got2.Data) != `{"pg":2}` {
		t.Fatalf("upsert did not update data: %s", got2.Data)
	}

	// DeactivateGeneric.
	if err := be.DeactivateGeneric(ctx, kind, sig); err != nil {
		t.Fatal(err)
	}
	got3, _ := be.GetGeneric(ctx, kind, sig)
	if got3.Active {
		t.Fatal("expected inactive after DeactivateGeneric")
	}

	// DeleteGeneric.
	if err := be.DeleteGeneric(ctx, kind, sig); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetGeneric(ctx, kind, sig); err != ErrBackendNotFound {
		t.Fatalf("want ErrBackendNotFound after Delete, got %v", err)
	}

	// Missing key.
	if _, err := be.GetGeneric(ctx, kind, "no-such-sig"); err != ErrBackendNotFound {
		t.Fatalf("want ErrBackendNotFound for unknown sig, got %v", err)
	}
}

// TestPGBackendRefreshRotateRevoke mirrors backend_mem_test.go's refresh assertions.
func TestPGBackendRefreshRotateRevoke(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	be := NewPGBackend(db)

	put := func(sig, reqID, sess, sub string) {
		t.Helper()
		t.Cleanup(func() { cleanupRefresh(ctx, db, sig) })
		if err := be.PutRefresh(ctx, sig, RefreshRecord{
			RequestID: reqID, ClientID: "c1", Subject: sub, SessionID: sess,
			Active: true, ExpiresAt: time.Now().Add(time.Hour), Data: []byte("{}"),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// DeactivateRefresh -> ErrBackendInactive.
	put("rt-pg-1", "req-A", "sess-1", "sub-1")
	if err := be.DeactivateRefresh(ctx, "rt-pg-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt-pg-1"); err != ErrBackendInactive {
		t.Fatalf("want ErrBackendInactive, got %v", err)
	}

	// RevokeRefreshByRequestID removes all tokens sharing a request_id.
	put("rt-pg-2", "req-B", "sess-2", "sub-1")
	put("rt-pg-3", "req-B", "sess-2", "sub-1")
	if err := be.RevokeRefreshByRequestID(ctx, "req-B"); err != nil {
		t.Fatal(err)
	}
	for _, sig := range []string{"rt-pg-2", "rt-pg-3"} {
		if _, err := be.GetRefresh(ctx, sig); err == nil {
			t.Fatalf("%s should be gone after RevokeRefreshByRequestID", sig)
		}
	}

	// RevokeRefreshBySession removes all tokens sharing a session_id.
	put("rt-pg-4", "req-C", "sess-3", "sub-2")
	put("rt-pg-5", "req-D", "sess-3", "sub-2")
	if err := be.RevokeRefreshBySession(ctx, "sess-3"); err != nil {
		t.Fatal(err)
	}
	for _, sig := range []string{"rt-pg-4", "rt-pg-5"} {
		if _, err := be.GetRefresh(ctx, sig); err == nil {
			t.Fatalf("%s should be gone after RevokeRefreshBySession", sig)
		}
	}

	// RevokeRefreshBySubject removes all tokens for a subject.
	put("rt-pg-6", "req-E", "sess-4", "sub-9")
	if err := be.RevokeRefreshBySubject(ctx, "sub-9"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt-pg-6"); err == nil {
		t.Fatal("rt-pg-6 should be gone after RevokeRefreshBySubject")
	}

	// DeleteRefresh.
	put("rt-pg-7", "req-F", "sess-5", "sub-3")
	if err := be.DeleteRefresh(ctx, "rt-pg-7"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt-pg-7"); err != ErrBackendNotFound {
		t.Fatalf("want ErrBackendNotFound after DeleteRefresh, got %v", err)
	}

	// Missing key.
	if _, err := be.GetRefresh(ctx, "no-such"); err != ErrBackendNotFound {
		t.Fatalf("want ErrBackendNotFound for unknown sig, got %v", err)
	}
}

// TestPGBackendTransactions verifies that Rollback discards and Commit persists.
func TestPGBackendTransactions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	be := NewPGBackend(db)

	const sigRollback = "rt-tx-rollback"
	const sigCommit = "rt-tx-commit"
	t.Cleanup(func() {
		cleanupRefresh(ctx, db, sigRollback)
		cleanupRefresh(ctx, db, sigCommit)
	})

	makeRec := func(sig string) RefreshRecord {
		return RefreshRecord{
			RequestID: "req-tx", ClientID: "c1", Subject: "sub-tx",
			SessionID: "sess-tx", Active: true,
			ExpiresAt: time.Now().Add(time.Hour), Data: []byte("{}"),
		}
	}

	// Rollback: row must not be visible outside the tx.
	ctx2, err := be.BeginTX(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := be.PutRefresh(ctx2, sigRollback, makeRec(sigRollback)); err != nil {
		_ = be.Rollback(ctx2)
		t.Fatal(err)
	}
	if err := be.Rollback(ctx2); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, sigRollback); err != ErrBackendNotFound {
		t.Fatalf("rollback: want ErrBackendNotFound, got %v", err)
	}

	// Commit: row must be visible outside the tx.
	ctx3, err := be.BeginTX(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := be.PutRefresh(ctx3, sigCommit, makeRec(sigCommit)); err != nil {
		_ = be.Rollback(ctx3)
		t.Fatal(err)
	}
	if err := be.Commit(ctx3); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, sigCommit); err != nil {
		t.Fatalf("commit: want success, got %v", err)
	}
}
