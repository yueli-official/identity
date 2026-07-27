//go:build integration

package dao_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yueli-official/identity/internal/dao"
	"github.com/yueli-official/identity/internal/repo"
)

// TestPGOAuthRoundTrip exercises the PG OAuthRepo against a real database.
// Requires TEST_PG_LINK and migrations applied (run with -tags=integration).
func TestPGOAuthRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	// CreateOAuthIdentity → GetByProviderUID round-trips.
	id, err := d.CreateOAuthIdentity(ctx, repo.NewOAuthIdentityInput{
		Email: "oauth-it@pg.com", EmailVerified: true, DisplayName: "OAuth IT",
		Provider: "google", ProviderUID: "it-sub-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", id.ID).Delete()
	}()

	got, err := d.GetByProviderUID(ctx, "google", "it-sub-1")
	if err != nil {
		t.Fatalf("GetByProviderUID: %v", err)
	}
	if got.ID != id.ID || got.Email != "oauth-it@pg.com" || !got.EmailVerified {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// Missing link → ErrIdentityMissing.
	if _, err := d.GetByProviderUID(ctx, "google", "nope-uid"); !errors.Is(err, repo.ErrIdentityMissing) {
		t.Fatalf("want ErrIdentityMissing, got %v", err)
	}

	// LinkOAuthCredential on an existing (password) identity resolves via GetByProviderUID.
	base, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "oauth-link-it@pg.com", DisplayName: "Link IT", PasswordHash: "h",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", base.ID).Delete()
	}()
	if err := d.LinkOAuthCredential(ctx, base.ID, "google", "it-sub-2", "oauth-link-it@pg.com", true); err != nil {
		t.Fatalf("LinkOAuthCredential: %v", err)
	}
	linked, err := d.GetByProviderUID(ctx, "google", "it-sub-2")
	if err != nil || linked.ID != base.ID {
		t.Fatalf("link should resolve to base identity: %v %#v", err, linked)
	}

	// Duplicate (provider, uid) → ErrProviderUIDTaken.
	if err := d.LinkOAuthCredential(ctx, base.ID, "google", "it-sub-2", "oauth-link-it@pg.com", true); !errors.Is(err, repo.ErrProviderUIDTaken) {
		t.Fatalf("want ErrProviderUIDTaken on duplicate, got %v", err)
	}
}
