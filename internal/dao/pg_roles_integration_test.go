//go:build integration

package dao_test

import (
	"context"
	"errors"
	"testing"

	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/repo"
)

// TestPGRoleRoundTrip exercises the PG RoleRepo against a real database:
// grant/get/revoke round-trip, unknown-role error, and idempotent grant.
// Requires TEST_PG_LINK and migrations applied (run with -tags=integration).
func TestPGRoleRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	// A real identity row is required to satisfy the identity_roles FK.
	id, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "roles-it@pg.com", DisplayName: "Roles IT", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// ON DELETE CASCADE on identity_roles cleans up the grants too.
		_, _ = db.Model("identities").Ctx(ctx).Where("id", id.ID).Delete()
	}()

	// No grants yet → empty (len 0) slice.
	got, err := d.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatalf("GetRoles (empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no roles, got %v", got)
	}

	// Unknown slug → ErrUnknownRole, and nothing is granted.
	if err := d.GrantRole(ctx, id.ID, "nope"); !errors.Is(err, repo.ErrUnknownRole) {
		t.Fatalf("want ErrUnknownRole, got %v", err)
	}

	// Grant the seeded catalog roles.
	if err := d.GrantRole(ctx, id.ID, "user"); err != nil {
		t.Fatalf("GrantRole user: %v", err)
	}
	if err := d.GrantRole(ctx, id.ID, "admin"); err != nil {
		t.Fatalf("GrantRole admin: %v", err)
	}

	// Re-granting an existing pair is an idempotent no-op (no error).
	if err := d.GrantRole(ctx, id.ID, "user"); err != nil {
		t.Fatalf("idempotent GrantRole user: %v", err)
	}

	// GetRoles returns the slugs sorted ascending, without duplicates.
	got, err = d.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatalf("GetRoles: %v", err)
	}
	if len(got) != 2 || got[0] != "admin" || got[1] != "user" {
		t.Fatalf("want sorted [admin user], got %v", got)
	}

	// Revoke one role; the other remains.
	if err := d.RevokeRole(ctx, id.ID, "user"); err != nil {
		t.Fatalf("RevokeRole user: %v", err)
	}
	got, err = d.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatalf("GetRoles after revoke: %v", err)
	}
	if len(got) != 1 || got[0] != "admin" {
		t.Fatalf("want [admin], got %v", got)
	}

	// Revoking a pair the identity does not have is a no-op (no error).
	if err := d.RevokeRole(ctx, id.ID, "user"); err != nil {
		t.Fatalf("no-op RevokeRole: %v", err)
	}
}
