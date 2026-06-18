//go:build integration

package dao_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/repo"
)

// TestPGAuditRoundTrip exercises the PG AuditRepo against a real database.
// Requires TEST_PG_LINK and migration 0007_audit_logs applied.
// Run with: go test -tags=integration ./internal/dao/ -run Audit
func TestPGAuditRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	actorID := uuid.NewString()
	targetID := uuid.NewString()
	event := "test.audit." + uuid.NewString()

	// --- insert → query round-trip ---
	row := repo.AuditRow{
		Event:      event,
		ActorID:    actorID,
		TargetID:   targetID,
		ActorEmail: "actor@example.com",
		IP:         "127.0.0.1",
		UserAgent:  "Go-test/1.0",
		ClientID:   "demo-web",
		RequestID:  "test-req-" + uuid.NewString(),
		Result:     "success",
		Detail:     map[string]any{"role": "admin"},
	}
	if err := d.InsertAudit(ctx, row); err != nil {
		t.Fatalf("InsertAudit: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("audit_logs").Ctx(ctx).Where("event", event).Delete()
	})

	// Query by event should return the inserted row.
	rows, err := d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 10})
	if err != nil {
		t.Fatalf("QueryAudit by event: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]

	// ID and OccurredAt are DB-generated; just assert non-zero.
	if got.ID == 0 {
		t.Error("want non-zero ID")
	}
	if got.OccurredAt.IsZero() {
		t.Error("want non-zero OccurredAt")
	}

	// Assert all explicitly-set fields.
	if got.Event != event {
		t.Errorf("Event: want %q, got %q", event, got.Event)
	}
	if got.ActorID != actorID {
		t.Errorf("ActorID: want %q, got %q", actorID, got.ActorID)
	}
	if got.TargetID != targetID {
		t.Errorf("TargetID: want %q, got %q", targetID, got.TargetID)
	}
	if got.ActorEmail != "actor@example.com" {
		t.Errorf("ActorEmail: want %q, got %q", "actor@example.com", got.ActorEmail)
	}
	if got.IP != "127.0.0.1" {
		t.Errorf("IP: want %q, got %q", "127.0.0.1", got.IP)
	}
	if got.UserAgent != "Go-test/1.0" {
		t.Errorf("UserAgent: want %q, got %q", "Go-test/1.0", got.UserAgent)
	}
	if got.ClientID != "demo-web" {
		t.Errorf("ClientID: want %q, got %q", "demo-web", got.ClientID)
	}
	if got.RequestID != row.RequestID {
		t.Errorf("RequestID: want %q, got %q", row.RequestID, got.RequestID)
	}
	if got.Result != "success" {
		t.Errorf("Result: want %q, got %q", "success", got.Result)
	}

	// Detail map round-trip.
	want := map[string]any{"role": "admin"}
	if !reflect.DeepEqual(got.Detail, want) {
		t.Errorf("Detail: want %v, got %v", want, got.Detail)
	}
}

// TestPGAuditQueryByIdentity verifies that QueryAudit with IdentityID matches
// both actor_identity_id and target_identity_id rows.
func TestPGAuditQueryByIdentity(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	sharedID := uuid.NewString()
	otherID := uuid.NewString()
	eventPrefix := "test.audit.byid." + uuid.NewString()

	// Row where sharedID is actor.
	rowActor := repo.AuditRow{
		Event:    eventPrefix + ".actor",
		ActorID:  sharedID,
		TargetID: otherID,
		Result:   "success",
		Detail:   map[string]any{},
	}
	// Row where sharedID is target.
	rowTarget := repo.AuditRow{
		Event:    eventPrefix + ".target",
		ActorID:  otherID,
		TargetID: sharedID,
		Result:   "success",
		Detail:   map[string]any{},
	}
	// Row that involves neither.
	rowOther := repo.AuditRow{
		Event:  eventPrefix + ".other",
		Result: "success",
		Detail: map[string]any{},
	}

	for _, r := range []repo.AuditRow{rowActor, rowTarget, rowOther} {
		if err := d.InsertAudit(ctx, r); err != nil {
			t.Fatalf("InsertAudit: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(ctx,
			"DELETE FROM audit_logs WHERE event LIKE $1",
			eventPrefix+"%",
		)
	})

	rows, err := d.QueryAudit(ctx, repo.AuditFilter{IdentityID: sharedID, Limit: 50})
	if err != nil {
		t.Fatalf("QueryAudit by identity: %v", err)
	}
	// Only 2 of the 3 rows involve sharedID.
	if len(rows) != 2 {
		t.Fatalf("want 2 rows for identity filter, got %d", len(rows))
	}
}

// TestPGAuditQueryByEvent verifies event-based filtering.
func TestPGAuditQueryByEvent(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	eventA := "test.audit.event." + uuid.NewString()
	eventB := "test.audit.event." + uuid.NewString()

	for i := 0; i < 3; i++ {
		if err := d.InsertAudit(ctx, repo.AuditRow{Event: eventA, Result: "success", Detail: map[string]any{}}); err != nil {
			t.Fatalf("InsertAudit eventA: %v", err)
		}
	}
	if err := d.InsertAudit(ctx, repo.AuditRow{Event: eventB, Result: "success", Detail: map[string]any{}}); err != nil {
		t.Fatalf("InsertAudit eventB: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("audit_logs").Ctx(ctx).WhereIn("event", []string{eventA, eventB}).Delete()
	})

	rows, err := d.QueryAudit(ctx, repo.AuditFilter{Event: eventA, Limit: 50})
	if err != nil {
		t.Fatalf("QueryAudit by event: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows for eventA, got %d", len(rows))
	}
}

// TestPGAuditLimitOffset verifies limit and offset clamping.
func TestPGAuditLimitOffset(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	event := "test.audit.limit." + uuid.NewString()
	for i := 0; i < 5; i++ {
		if err := d.InsertAudit(ctx, repo.AuditRow{Event: event, Result: "success", Detail: map[string]any{}}); err != nil {
			t.Fatalf("InsertAudit: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Model("audit_logs").Ctx(ctx).Where("event", event).Delete()
	})

	// Limit 2 → 2 rows.
	rows, err := d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 2})
	if err != nil {
		t.Fatalf("QueryAudit limit=2: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (limit=2), got %d", len(rows))
	}

	// Limit 0 → default 50, so all 5 rows are returned.
	rows, err = d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 0})
	if err != nil {
		t.Fatalf("QueryAudit limit=0: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("want 5 rows (limit=0→default), got %d", len(rows))
	}
	// Results must be newest-first (ORDER BY id DESC): IDs strictly decreasing.
	// Catches a regression flipping the order to ASC.
	for i := 1; i < len(rows); i++ {
		if rows[i-1].ID <= rows[i].ID {
			t.Fatalf("rows not newest-first: rows[%d].ID=%d <= rows[%d].ID=%d",
				i-1, rows[i-1].ID, i, rows[i].ID)
		}
	}

	// Offset 3 → 2 remaining rows.
	rows, err = d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 50, Offset: 3})
	if err != nil {
		t.Fatalf("QueryAudit offset=3: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (offset=3), got %d", len(rows))
	}

	// Negative limit → default 50.
	rows, err = d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: -1})
	if err != nil {
		t.Fatalf("QueryAudit limit=-1: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("want 5 rows (limit=-1→default), got %d", len(rows))
	}
}

// TestPGAuditNullUUIDs verifies that rows with empty ActorID/TargetID (login.failure style)
// insert as NULL UUIDs in Postgres and read back as empty strings.
func TestPGAuditNullUUIDs(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	event := "test.audit.nulluuid." + uuid.NewString()
	row := repo.AuditRow{
		Event:  event,
		// ActorID and TargetID intentionally empty → must become NULL in PG.
		Result: "failure",
		Detail: map[string]any{"reason": "bad credentials"},
	}
	if err := d.InsertAudit(ctx, row); err != nil {
		t.Fatalf("InsertAudit (null UUIDs): %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("audit_logs").Ctx(ctx).Where("event", event).Delete()
	})

	rows, err := d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 10})
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.ActorID != "" {
		t.Errorf("ActorID: want empty string (NULL roundtrip), got %q", got.ActorID)
	}
	if got.TargetID != "" {
		t.Errorf("TargetID: want empty string (NULL roundtrip), got %q", got.TargetID)
	}
	if got.Result != "failure" {
		t.Errorf("Result: want %q, got %q", "failure", got.Result)
	}

	// Detail round-trip.
	want := map[string]any{"reason": "bad credentials"}
	if !reflect.DeepEqual(got.Detail, want) {
		t.Errorf("Detail: want %v, got %v", want, got.Detail)
	}
}

// TestPGAuditResultDefault verifies that an empty Result defaults to "success".
func TestPGAuditResultDefault(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	event := "test.audit.resultdefault." + uuid.NewString()
	if err := d.InsertAudit(ctx, repo.AuditRow{Event: event, Detail: map[string]any{}}); err != nil {
		t.Fatalf("InsertAudit: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("audit_logs").Ctx(ctx).Where("event", event).Delete()
	})

	rows, err := d.QueryAudit(ctx, repo.AuditFilter{Event: event, Limit: 10})
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Result != "success" {
		t.Errorf("Result default: want %q, got %q", "success", rows[0].Result)
	}
}
