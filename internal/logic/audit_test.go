package logic_test

import (
	"context"
	"errors"
	"testing"

	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newSvcWithStore builds a logic.Service backed by the provided store.
func newSvcWithStore(st repo.Store) *logic.Service {
	return logic.New(st, logic.DefaultConfig())
}

// actorCtx injects actor metadata (IP only here; extend as needed).
func actorCtx(ip string) context.Context {
	return actor.With(context.Background(), actor.Actor{IP: ip})
}

// hasEvent returns true if any audit row in rows has the given event name.
func hasEvent(rows []repo.AuditRow, event string) bool {
	for _, r := range rows {
		if r.Event == event {
			return true
		}
	}
	return false
}

// findEvent returns the first audit row with the given event name, or zero value + false.
func findEvent(rows []repo.AuditRow, event string) (repo.AuditRow, bool) {
	for _, r := range rows {
		if r.Event == event {
			return r, true
		}
	}
	return repo.AuditRow{}, false
}

// allAudit reads all audit rows from store without filter.
func allAudit(t *testing.T, st repo.Store) []repo.AuditRow {
	t.Helper()
	rows, err := st.QueryAudit(context.Background(), repo.AuditFilter{Limit: 200})
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	return rows
}

// ---------------------------------------------------------------------------
// failingAuditStore wraps a Store and makes InsertAudit always return an error.
// Used to verify best-effort behaviour (business ops must not fail).
// ---------------------------------------------------------------------------

type failingAuditStore struct {
	repo.Store
}

func (f *failingAuditStore) InsertAudit(_ context.Context, _ repo.AuditRow) error {
	return errors.New("forced audit failure")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAudit_LoginSuccess checks that a successful login records login.success
// with the correct ActorID and the IP from the injected actor context.
func TestAudit_LoginSuccess(t *testing.T) {
	m := repo.NewMemory()
	svc := newSvcWithStore(m)
	ctx := actorCtx("9.9.9.9")

	// seed user
	id, err := svc.Register(ctx, logic.RegisterInput{Email: "audit@example.com", Password: "longenough123"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Login(ctx, logic.LoginInput{Email: "audit@example.com", Password: "longenough123", IP: "9.9.9.9"})
	if err != nil {
		t.Fatal(err)
	}

	rows := allAudit(t, m)
	row, ok := findEvent(rows, logic.EvLoginSuccess)
	if !ok {
		t.Fatalf("expected %q audit event, got events: %v", logic.EvLoginSuccess, eventNames(rows))
	}
	if row.ActorID != id.ID {
		t.Errorf("ActorID: want %q, got %q", id.ID, row.ActorID)
	}
	if row.IP != "9.9.9.9" {
		t.Errorf("IP: want 9.9.9.9, got %q", row.IP)
	}
	if row.Result != "success" {
		t.Errorf("Result: want success, got %q", row.Result)
	}
}

// TestAudit_LoginFailure_BadPassword checks that a wrong-password attempt records
// login.failure with Result="failure" and Detail["reason"]=="bad_password".
func TestAudit_LoginFailure_BadPassword(t *testing.T) {
	m := repo.NewMemory()
	svc := newSvcWithStore(m)
	ctx := actorCtx("1.2.3.4")

	if _, err := svc.Register(ctx, logic.RegisterInput{Email: "fail@example.com", Password: "longenough123"}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Login(ctx, logic.LoginInput{Email: "fail@example.com", Password: "wrongpass", IP: "1.2.3.4"})
	if err == nil {
		t.Fatal("expected login error")
	}

	rows := allAudit(t, m)
	row, ok := findEvent(rows, logic.EvLoginFailure)
	if !ok {
		t.Fatalf("expected %q audit event, got events: %v", logic.EvLoginFailure, eventNames(rows))
	}
	if row.Result != "failure" {
		t.Errorf("Result: want failure, got %q", row.Result)
	}
	if row.Detail["reason"] != "bad_password" {
		t.Errorf("Detail[reason]: want bad_password, got %v", row.Detail["reason"])
	}
}

// TestAudit_LoginFailure_UnknownEmail checks that an unknown-email attempt records
// login.failure with Detail["reason"]=="unknown_email".
func TestAudit_LoginFailure_UnknownEmail(t *testing.T) {
	m := repo.NewMemory()
	svc := newSvcWithStore(m)
	ctx := actorCtx("5.5.5.5")

	_, err := svc.Login(ctx, logic.LoginInput{Email: "ghost@example.com", Password: "anything", IP: "5.5.5.5"})
	if err == nil {
		t.Fatal("expected login error")
	}

	rows := allAudit(t, m)
	row, ok := findEvent(rows, logic.EvLoginFailure)
	if !ok {
		t.Fatalf("expected %q audit event, got events: %v", logic.EvLoginFailure, eventNames(rows))
	}
	if row.Result != "failure" {
		t.Errorf("Result: want failure, got %q", row.Result)
	}
	if row.Detail["reason"] != "unknown_email" {
		t.Errorf("Detail[reason]: want unknown_email, got %v", row.Detail["reason"])
	}
}

// TestAudit_Register checks that Register records identity.register and
// role.default_granted.
func TestAudit_Register(t *testing.T) {
	m := repo.NewMemory()
	svc := newSvcWithStore(m)
	ctx := context.Background()

	id, err := svc.Register(ctx, logic.RegisterInput{Email: "reg@example.com", Password: "longenough123"})
	if err != nil {
		t.Fatal(err)
	}

	rows := allAudit(t, m)

	// identity.register
	regRow, ok := findEvent(rows, logic.EvRegister)
	if !ok {
		t.Fatalf("expected %q audit event, got events: %v", logic.EvRegister, eventNames(rows))
	}
	if regRow.ActorID != id.ID || regRow.TargetID != id.ID {
		t.Errorf("register ActorID/TargetID: want %q, got actor=%q target=%q", id.ID, regRow.ActorID, regRow.TargetID)
	}

	// role.default_granted
	roleRow, ok := findEvent(rows, logic.EvRoleDefaultGranted)
	if !ok {
		t.Fatalf("expected %q audit event, got events: %v", logic.EvRoleDefaultGranted, eventNames(rows))
	}
	if roleRow.Detail["role"] != logic.DefaultRole {
		t.Errorf("role.default_granted Detail[role]: want %q, got %v", logic.DefaultRole, roleRow.Detail["role"])
	}
	if roleRow.Detail["best_effort"] != true {
		t.Errorf("role.default_granted Detail[best_effort]: want true, got %v", roleRow.Detail["best_effort"])
	}
}

// TestAudit_RoleGranted checks that GrantRole records role.granted with correct
// TargetID and Detail["role"].
func TestAudit_RoleGranted(t *testing.T) {
	m := repo.NewMemory()
	svc := newSvcWithStore(m)
	ctx := context.Background()

	id, err := svc.Register(ctx, logic.RegisterInput{Email: "admin@example.com", Password: "longenough123"})
	if err != nil {
		t.Fatal(err)
	}

	// clear audit rows from register so we start clean
	preCount := len(allAudit(t, m))

	if err := svc.GrantRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal(err)
	}

	rows := allAudit(t, m)
	// Only look at rows added after pre-count
	newRows := rows[:len(rows)-preCount] // QueryAudit returns newest-first
	row, ok := findEvent(newRows, logic.EvRoleGranted)
	if !ok {
		t.Fatalf("expected %q audit event in new rows, got: %v", logic.EvRoleGranted, eventNames(newRows))
	}
	if row.TargetID != id.ID {
		t.Errorf("TargetID: want %q, got %q", id.ID, row.TargetID)
	}
	if row.Detail["role"] != "admin" {
		t.Errorf("Detail[role]: want admin, got %v", row.Detail["role"])
	}
}

// TestAudit_BestEffort verifies that an InsertAudit failure does NOT cause
// Login or Register to return an error.
func TestAudit_BestEffort(t *testing.T) {
	base := repo.NewMemory()
	failing := &failingAuditStore{Store: base}
	svc := newSvcWithStore(failing)
	ctx := context.Background()

	// Register must succeed even though audit fails.
	_, err := svc.Register(ctx, logic.RegisterInput{Email: "betest@example.com", Password: "longenough123"})
	if err != nil {
		t.Fatalf("Register with failing audit must not error: %v", err)
	}

	// Login must succeed even though audit fails.
	_, err = svc.Login(ctx, logic.LoginInput{Email: "betest@example.com", Password: "longenough123", IP: "1.1.1.1"})
	if err != nil {
		t.Fatalf("Login with failing audit must not error: %v", err)
	}
}

// eventNames extracts the event names from a slice for error messages.
func eventNames(rows []repo.AuditRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Event
	}
	return names
}
