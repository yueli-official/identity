package logic_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/pat"
	"platform/services/identity/internal/repo"
)

// newPATSvc creates a Service backed by the given store, with DefaultConfig and
// a controllable clock. It returns the service and a pointer whose value is read
// by the fake clock; callers advance time by writing to *nowPtr.
func newPATSvc(st repo.Store) (*logic.Service, *time.Time) {
	svc := logic.New(st, logic.DefaultConfig())
	t0 := time.Now()
	svc.SetNow(func() time.Time { return t0 })
	// Return pointer to local — caller must use advanceClock helper.
	// NOTE: closures capture variables by reference, but t0 is a value here.
	// We return the service separately so tests can advance via SetNow.
	// Actually re-implement using a mutable time pointer:
	return svc, &t0
}

// newPATSvcClk creates a Service with a mutable clock pointer.
func newPATSvcClk(st repo.Store) (*logic.Service, *time.Time) {
	now := time.Now()
	nowPtr := &now
	svc := logic.New(st, logic.DefaultConfig())
	svc.SetNow(func() time.Time { return *nowPtr })
	return svc, nowPtr
}

// advanceClock moves the fake clock forward by d.
func advanceClock(nowPtr *time.Time, d time.Duration) {
	*nowPtr = (*nowPtr).Add(d)
}

// patHash is a shortcut to derive-key then hash, using DefaultConfig.
func patHash(plaintext string) string {
	key := pat.DeriveKey(logic.DefaultConfig().PATHMACSecret)
	return pat.Hash(key, plaintext)
}

// ---------------------------------------------------------------------------
// Create success
// ---------------------------------------------------------------------------

func TestCreatePAT_Success(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	plaintext, row, err := svc.CreatePAT(ctx, "id-1", "my-token", []string{"read", "write"}, 0)
	if err != nil {
		t.Fatalf("CreatePAT: %v", err)
	}

	// plaintext must start with pat_
	if !strings.HasPrefix(plaintext, pat.Prefix) {
		t.Errorf("plaintext must start with %q, got %q", pat.Prefix, plaintext)
	}

	// row ID must be non-zero
	if row.ID == 0 {
		t.Error("row.ID must be non-zero after insert")
	}

	// TokenPrefix must be the first PrefixLen chars of plaintext
	if row.TokenPrefix != plaintext[:pat.PrefixLen] {
		t.Errorf("TokenPrefix: want %q, got %q", plaintext[:pat.PrefixLen], row.TokenPrefix)
	}

	// The stored hash must NOT equal the plaintext
	if row.TokenHash == plaintext {
		t.Error("TokenHash must not equal plaintext")
	}

	// GetPATByHash with the correct derived hash must find it
	found, ok, err2 := m.GetPATByHash(ctx, patHash(plaintext))
	if err2 != nil {
		t.Fatalf("GetPATByHash: %v", err2)
	}
	if !ok {
		t.Error("expected to find PAT by hash")
	}
	if found.ID != row.ID {
		t.Errorf("found.ID: want %d, got %d", row.ID, found.ID)
	}
}

// ---------------------------------------------------------------------------
// Create validation
// ---------------------------------------------------------------------------

func TestCreatePAT_EmptyName(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	_, _, err := svc.CreatePAT(context.Background(), "id-1", "", []string{"read"}, 0)
	if codeOfErr(err) != iderr.CodePATNameRequired {
		t.Errorf("empty name: want PATNameRequired, got %v", err)
	}
}

func TestCreatePAT_WhitespaceName(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	_, _, err := svc.CreatePAT(context.Background(), "id-1", "   ", []string{"read"}, 0)
	if codeOfErr(err) != iderr.CodePATNameRequired {
		t.Errorf("whitespace name: want PATNameRequired, got %v", err)
	}
}

func TestCreatePAT_EmptyScopes(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	_, _, err := svc.CreatePAT(context.Background(), "id-1", "tok", []string{}, 0)
	if codeOfErr(err) != iderr.CodePATScopesRequired {
		t.Errorf("empty scopes: want PATScopesRequired, got %v", err)
	}
}

func TestCreatePAT_ScopeWithSpace(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	_, _, err := svc.CreatePAT(context.Background(), "id-1", "tok", []string{"read write"}, 0)
	if codeOfErr(err) != iderr.CodePATScopeInvalid {
		t.Errorf("scope with space: want PATScopeInvalid, got %v", err)
	}
}

func TestCreatePAT_TooManyScopes(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	scopes := make([]string, 51)
	for i := range scopes {
		scopes[i] = "scope"
	}
	_, _, err := svc.CreatePAT(context.Background(), "id-1", "tok", scopes, 0)
	if codeOfErr(err) != iderr.CodePATScopeInvalid {
		t.Errorf(">50 scopes: want PATScopeInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Cap (PATMaxPerUser)
// ---------------------------------------------------------------------------

func TestCreatePAT_LimitReached(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()
	cfg := logic.DefaultConfig()

	for i := 0; i < cfg.PATMaxPerUser; i++ {
		_, _, err := svc.CreatePAT(ctx, "id-cap", "tok", []string{"read"}, 0)
		if err != nil {
			t.Fatalf("create #%d: %v", i+1, err)
		}
	}

	_, _, err := svc.CreatePAT(ctx, "id-cap", "extra", []string{"read"}, 0)
	if codeOfErr(err) != iderr.CodePATLimitReached {
		t.Errorf("over limit: want PATLimitReached, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Revoke
// ---------------------------------------------------------------------------

func TestRevokePAT_Success(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	_, row, err := svc.CreatePAT(ctx, "id-1", "tok", []string{"read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.RevokePAT(ctx, "id-1", row.ID); err != nil {
		t.Fatalf("RevokePAT: %v", err)
	}

	// listing should return empty
	rows, err := svc.ListPAT(ctx, "id-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty list after revoke, got %d rows", len(rows))
	}
}

func TestRevokePAT_WrongIdentity(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	_, row, err := svc.CreatePAT(ctx, "id-owner", "tok", []string{"read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = svc.RevokePAT(ctx, "id-other", row.ID)
	if codeOfErr(err) != iderr.CodePATNotFound {
		t.Errorf("wrong identity: want PATNotFound, got %v", err)
	}
}

func TestRevokePAT_ThenVerify(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	plaintext, row, err := svc.CreatePAT(ctx, "id-1", "tok", []string{"read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.RevokePAT(ctx, "id-1", row.ID); err != nil {
		t.Fatal(err)
	}

	_, err = svc.VerifyPAT(ctx, plaintext)
	if codeOfErr(err) != iderr.CodePATInvalid {
		t.Errorf("verify after revoke: want PATInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Verify
// ---------------------------------------------------------------------------

func TestVerifyPAT_Success(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	plaintext, _, err := svc.CreatePAT(ctx, "id-1", "tok", []string{"read", "write"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.VerifyPAT(ctx, plaintext)
	if err != nil {
		t.Fatalf("VerifyPAT: %v", err)
	}
	if result.IdentityID != "id-1" {
		t.Errorf("IdentityID: want id-1, got %q", result.IdentityID)
	}
	if len(result.Scopes) != 2 || result.Scopes[0] != "read" || result.Scopes[1] != "write" {
		t.Errorf("Scopes: want [read write], got %v", result.Scopes)
	}
}

func TestVerifyPAT_Garbage(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	_, err := svc.VerifyPAT(context.Background(), "not-a-real-token")
	if codeOfErr(err) != iderr.CodePATInvalid {
		t.Errorf("garbage: want PATInvalid, got %v", err)
	}
}

func TestVerifyPAT_Expired(t *testing.T) {
	m := repo.NewMemory()
	svc, nowPtr := newPATSvcClk(m)
	ctx := context.Background()

	// create with 1-day expiry
	plaintext, _, err := svc.CreatePAT(ctx, "id-1", "tok", []string{"read"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	// advance clock past expiry
	advanceClock(nowPtr, 25*time.Hour)

	_, err = svc.VerifyPAT(ctx, plaintext)
	if codeOfErr(err) != iderr.CodePATExpired {
		t.Errorf("expired: want PATExpired, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Throttle (last_used)
// ---------------------------------------------------------------------------

func TestVerifyPAT_ThrottleTouch(t *testing.T) {
	m := repo.NewMemory()
	svc, nowPtr := newPATSvcClk(m)
	ctx := context.Background()

	plaintext, _, err := svc.CreatePAT(ctx, "id-1", "tok", []string{"read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	// First verify: last_used should be written
	if _, err := svc.VerifyPAT(ctx, plaintext); err != nil {
		t.Fatalf("first verify: %v", err)
	}

	got1, ok, _ := m.GetPATByHash(ctx, patHash(plaintext))
	if !ok {
		t.Fatal("expected to find PAT")
	}
	if got1.LastUsedAt == nil {
		t.Fatal("LastUsedAt should be set after first verify")
	}
	firstLastUsed := *got1.LastUsedAt

	// Advance <1 minute: second verify should NOT update last_used
	advanceClock(nowPtr, 30*time.Second)
	if _, err := svc.VerifyPAT(ctx, plaintext); err != nil {
		t.Fatalf("second verify: %v", err)
	}

	got2, _, _ := m.GetPATByHash(ctx, patHash(plaintext))
	if got2.LastUsedAt == nil || !got2.LastUsedAt.Equal(firstLastUsed) {
		t.Errorf("LastUsedAt should be unchanged within 1 minute: was %v, got %v", firstLastUsed, got2.LastUsedAt)
	}

	// Advance >1 minute total: third verify SHOULD update last_used
	advanceClock(nowPtr, 2*time.Minute)
	if _, err := svc.VerifyPAT(ctx, plaintext); err != nil {
		t.Fatalf("third verify: %v", err)
	}

	got3, _, _ := m.GetPATByHash(ctx, patHash(plaintext))
	if got3.LastUsedAt == nil || got3.LastUsedAt.Equal(firstLastUsed) {
		t.Errorf("LastUsedAt should be updated after >1 minute: was %v, still %v", firstLastUsed, got3.LastUsedAt)
	}
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

func TestPAT_Audit(t *testing.T) {
	m := repo.NewMemory()
	svc, _ := newPATSvcClk(m)
	ctx := context.Background()

	_, row, err := svc.CreatePAT(ctx, "id-audit", "tok", []string{"read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.RevokePAT(ctx, "id-audit", row.ID); err != nil {
		t.Fatal(err)
	}

	rows := allAudit(t, m)

	created, ok := findEvent(rows, logic.EvPATCreated)
	if !ok {
		t.Fatalf("expected pat.created audit event, got: %v", eventNames(rows))
	}
	if created.ActorID != "id-audit" || created.TargetID != "id-audit" {
		t.Errorf("pat.created ActorID/TargetID: want id-audit, got actor=%q target=%q", created.ActorID, created.TargetID)
	}

	revoked, ok := findEvent(rows, logic.EvPATRevoked)
	if !ok {
		t.Fatalf("expected pat.revoked audit event, got: %v", eventNames(rows))
	}
	if revoked.ActorID != "id-audit" || revoked.TargetID != "id-audit" {
		t.Errorf("pat.revoked ActorID/TargetID: want id-audit, got actor=%q target=%q", revoked.ActorID, revoked.TargetID)
	}
}
