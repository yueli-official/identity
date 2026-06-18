package actor_test

import (
	"context"
	"testing"

	"platform/services/identity/internal/actor"
)

func TestWith_From_roundTrip(t *testing.T) {
	want := actor.Actor{
		IP:         "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		RequestID:  "req-abc-123",
		IdentityID: "user-xyz",
	}
	ctx := actor.With(context.Background(), want)
	got := actor.From(ctx)
	if got != want {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestFrom_emptyContext_returnsZero(t *testing.T) {
	got := actor.From(context.Background())
	if got != (actor.Actor{}) {
		t.Errorf("expected zero Actor from background ctx, got %+v", got)
	}
}

func TestFrom_nilDerivedContext_returnsZero(t *testing.T) {
	// A derived context that has no actor value should also return zero.
	ctx := context.WithValue(context.Background(), "some-other-key", "value")
	got := actor.From(ctx)
	if got != (actor.Actor{}) {
		t.Errorf("expected zero Actor from ctx without actor, got %+v", got)
	}
}

func TestWithIdentity_preservesExistingFields(t *testing.T) {
	base := actor.Actor{
		IP:        "1.2.3.4",
		UserAgent: "TestAgent/1.0",
		RequestID: "req-001",
	}
	ctx := actor.With(context.Background(), base)
	ctx = actor.WithIdentity(ctx, "u1")

	got := actor.From(ctx)
	if got.IP != "1.2.3.4" {
		t.Errorf("IP not preserved: got %q, want %q", got.IP, "1.2.3.4")
	}
	if got.UserAgent != "TestAgent/1.0" {
		t.Errorf("UserAgent not preserved: got %q, want %q", got.UserAgent, "TestAgent/1.0")
	}
	if got.RequestID != "req-001" {
		t.Errorf("RequestID not preserved: got %q, want %q", got.RequestID, "req-001")
	}
	if got.IdentityID != "u1" {
		t.Errorf("IdentityID not set: got %q, want %q", got.IdentityID, "u1")
	}
}

func TestWithIdentity_onEmptyContext_setsOnlyIdentityID(t *testing.T) {
	ctx := actor.WithIdentity(context.Background(), "u2")
	got := actor.From(ctx)
	if got.IdentityID != "u2" {
		t.Errorf("IdentityID not set: got %q, want %q", got.IdentityID, "u2")
	}
	// Other fields should be zero-value.
	if got.IP != "" || got.UserAgent != "" || got.RequestID != "" {
		t.Errorf("unexpected non-empty fields in zero Actor after WithIdentity: %+v", got)
	}
}
