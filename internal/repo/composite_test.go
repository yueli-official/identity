package repo_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/repo"
)

func TestNewCompositeIsNonNilAndUsable(t *testing.T) {
	ctx := context.Background()

	mem := repo.NewMemory()
	store := repo.NewComposite(mem, mem, mem)
	if store == nil {
		t.Fatal("NewComposite returned nil")
	}

	// Create an identity through the composite store and retrieve it.
	id, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email:        "composite@test.com",
		DisplayName:  "Composite User",
		PasswordHash: "hashedpw",
	})
	if err != nil {
		t.Fatalf("CreateIdentityWithProfile via composite: %v", err)
	}
	if id.ID == "" {
		t.Fatal("created identity has empty ID")
	}

	got, err := store.GetByEmail(ctx, "composite@test.com")
	if err != nil {
		t.Fatalf("GetByEmail via composite: %v", err)
	}
	if got.ID != id.ID {
		t.Fatalf("id mismatch: want %q, got %q", id.ID, got.ID)
	}
}

// Compile-time guarantee Composite satisfies the full Store surface.
var _ repo.Store = repo.Composite{}
