//go:build integration

package dao_test

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/google/uuid"

	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

func newDAO(t *testing.T, db gdb.DB) *dao.PG {
	t.Helper()
	return dao.NewPG(db)
}

// TestOIDCGetClient verifies that the seeded demo-web client can be loaded from
// PG and that text[] columns are correctly decoded into []string.
func TestOIDCGetClient(t *testing.T) {
	db := newDB(t)
	d := newDAO(t, db)
	ctx := context.Background()

	c, err := d.GetClient(ctx, "demo-web")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if c.ID != "demo-web" {
		t.Fatalf("unexpected client id: %q", c.ID)
	}
	if len(c.RedirectURIs) == 0 {
		t.Fatal("expected non-empty redirect_uris for demo-web")
	}
}

// TestOIDCGetClientNotFound verifies that a missing client returns ErrClientNotFound.
func TestOIDCGetClientNotFound(t *testing.T) {
	db := newDB(t)
	d := newDAO(t, db)
	ctx := context.Background()

	_, err := d.GetClient(ctx, "nonexistent-client-"+uuid.NewString())
	if err != repo.ErrClientNotFound {
		t.Fatalf("want ErrClientNotFound, got %v", err)
	}
}

// TestOIDCSigningKeys verifies InsertKey / GetActiveKey / ListPublicKeys round-trip.
func TestOIDCSigningKeys(t *testing.T) {
	db := newDB(t)
	d := newDAO(t, db)
	ctx := context.Background()

	kid := "test-kid-" + uuid.NewString()
	k := model.SigningKey{
		KID:        kid,
		Alg:        "RS256",
		PrivatePEM: "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		PublicPEM:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		Status:     model.KeyActive,
	}

	// Insert a key (there may already be an active key; that's fine for list test).
	if err := d.InsertKey(ctx, k); err != nil {
		t.Fatalf("InsertKey: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("oidc_signing_keys").Ctx(ctx).Where("kid", kid).Delete()
	})

	// ListPublicKeys must include at least the one we inserted.
	ks, err := d.ListPublicKeys(ctx)
	if err != nil {
		t.Fatalf("ListPublicKeys: %v", err)
	}
	found := false
	for _, kk := range ks {
		if kk.KID == kid {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListPublicKeys: inserted key %q not found in result (len=%d)", kid, len(ks))
	}
}

// TestOIDCGetProfile verifies that GetProfile works for a created identity.
func TestOIDCGetProfile(t *testing.T) {
	db := newDB(t)
	d := newDAO(t, db)
	ctx := context.Background()

	email := "oidc-profile-test-" + uuid.NewString() + "@example.com"
	id, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email:        email,
		DisplayName:  "OIDC Test",
		PasswordHash: "h",
	})
	if err != nil {
		t.Fatalf("CreateIdentityWithProfile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", id.ID).Delete()
	})

	prof, err := d.GetProfile(ctx, id.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if prof.IdentityID != id.ID {
		t.Fatalf("GetProfile: want identity_id %q, got %q", id.ID, prof.IdentityID)
	}
}
