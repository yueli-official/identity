//go:build integration

package dao_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/repo"
)

// TestPGVerificationRoundTrip exercises the PG VerificationRepo + setters against
// a real database. Requires TEST_PG_LINK and migrations applied (run with
// -tags=integration).
func TestPGVerificationRoundTrip(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()

	id, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "verif-it@pg.com", DisplayName: "Verif IT", PasswordHash: "old",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", id.ID).Delete()
	}()

	// CreateVerification → ConsumeVerification round-trips and is single-use.
	if err := d.CreateVerification(ctx, repo.NewVerificationInput{
		IdentityID: id.ID, Email: id.Email, Purpose: repo.PurposeVerifyEmail,
		TokenHash: "it-hash-1", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	rec, err := d.ConsumeVerification(ctx, "it-hash-1", repo.PurposeVerifyEmail)
	if err != nil || rec.IdentityID != id.ID || rec.Email != id.Email {
		t.Fatalf("first consume: %+v %v", rec, err)
	}
	if _, err := d.ConsumeVerification(ctx, "it-hash-1", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("second consume must fail single-use, got %v", err)
	}

	// Purpose mismatch → invalid.
	if err := d.CreateVerification(ctx, repo.NewVerificationInput{
		IdentityID: id.ID, Email: id.Email, Purpose: repo.PurposePasswordReset,
		TokenHash: "it-hash-2", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ConsumeVerification(ctx, "it-hash-2", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("purpose mismatch must fail, got %v", err)
	}

	// Expired token → invalid.
	if err := d.CreateVerification(ctx, repo.NewVerificationInput{
		IdentityID: id.ID, Email: id.Email, Purpose: repo.PurposeVerifyEmail,
		TokenHash: "it-hash-3", ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ConsumeVerification(ctx, "it-hash-3", repo.PurposeVerifyEmail); !errors.Is(err, repo.ErrVerificationInvalid) {
		t.Fatalf("expired must fail, got %v", err)
	}

	// SetEmailVerified round-trips.
	if err := d.SetEmailVerified(ctx, id.ID, true); err != nil {
		t.Fatalf("SetEmailVerified: %v", err)
	}
	got, err := d.GetByID(ctx, id.ID)
	if err != nil || !got.EmailVerified {
		t.Fatalf("email_verified not set: %v %#v", err, got)
	}

	// UpdatePasswordHash round-trips.
	if err := d.UpdatePasswordHash(ctx, id.ID, "newhash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	h, err := d.GetPasswordHash(ctx, id.ID)
	if err != nil || h != "newhash" {
		t.Fatalf("password not updated: %q %v", h, err)
	}
}
