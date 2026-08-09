//go:build integration

package dao_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/dao"
	"github.com/yueli-official/identity/internal/repo"
)

func TestPGTOTPEnrollmentLifecycle(t *testing.T) {
	db := newDB(t)
	store := dao.NewPG(db)
	ctx := context.Background()

	identity, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: "mfa-" + identifier.MustNew().String() + "@pg.test", PasswordHash: "password-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	session := authentication.Session{
		ID: identifier.MustNew().String(), IdentityID: identity.ID,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(time.Hour),
		Authentication: authentication.Password(identifier.MustNew().String(), now),
	}
	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
		t.Fatal(err)
	}

	expiresAt := now.Add(5 * time.Minute)
	authenticator := authentication.TOTPAuthenticator{
		ID: identifier.MustNew().String(), IdentityID: identity.ID, Label: "Authenticator",
		SecretCiphertext: make([]byte, 32), KeyVersion: 1,
		Algorithm: "SHA1", Digits: 6, PeriodSeconds: 30, Status: "pending",
		BindingSessionID: session.ID, EnrollmentExpiresAt: &expiresAt,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreatePendingTOTP(ctx, authenticator); err != nil {
		t.Fatalf("CreatePendingTOTP() error = %v", err)
	}
	got, err := store.GetTOTP(ctx, identity.ID, authenticator.ID)
	if err != nil || got.BindingSessionID != session.ID {
		t.Fatalf("GetTOTP() = %+v, %v", got, err)
	}
	if err := store.RecordTOTPFailure(ctx, authenticator.ID, 5); err != nil {
		t.Fatal(err)
	}

	codes := make([]authentication.RecoveryCode, 10)
	for index := range codes {
		codes[index] = authentication.RecoveryCode{
			ID: identifier.MustNew().String(), Digest: bytesOf(byte(index+1), 32),
		}
	}
	if err := store.ActivateTOTP(
		ctx, authenticator, session.ID, 123,
		identifier.MustNew().String(), codes, now.Add(time.Second),
	); err != nil {
		t.Fatalf("ActivateTOTP() error = %v", err)
	}
	if count, err := store.CountActiveTOTP(ctx, identity.ID); err != nil || count != 1 {
		t.Fatalf("CountActiveTOTP() = %d, %v; want 1", count, err)
	}
	listed, err := store.ListTOTP(ctx, identity.ID)
	if err != nil || len(listed) != 1 || listed[0].Status != "active" {
		t.Fatalf("ListTOTP() = %+v, %v", listed, err)
	}

	loginTransaction := authentication.AuthenticationTransaction{
		ID: identifier.MustNew().String(), Kind: "mfa_login", IdentityID: identity.ID,
		Requirement: []byte(`{"minimumLevel":"aal2"}`), State: []byte(`{}`),
		ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now,
	}
	if err := store.CreateAuthenticationTransaction(ctx, loginTransaction); err != nil {
		t.Fatal(err)
	}
	loadedTransaction, err := store.GetAuthenticationTransaction(ctx, loginTransaction.ID)
	if err != nil || loadedTransaction.IdentityID != identity.ID {
		t.Fatalf("GetAuthenticationTransaction() = %+v, %v", loadedTransaction, err)
	}
	mfaSession := authentication.Session{
		ID: identifier.MustNew().String(), IdentityID: identity.ID,
		CreatedAt: now.Add(2 * time.Second), LastSeen: now.Add(2 * time.Second),
		ExpiresAt: now.Add(time.Hour),
		Authentication: authentication.MultiFactor(
			authentication.Password(identifier.MustNew().String(), now),
			identifier.MustNew().String(), now.Add(2*time.Second), authenticator.ID,
		),
	}
	if err := store.CompleteTOTPLogin(
		ctx, loadedTransaction, authenticator.ID, 124, mfaSession,
	); err != nil {
		t.Fatalf("CompleteTOTPLogin() error = %v", err)
	}
	if err := store.CompleteTOTPLogin(
		ctx, loadedTransaction, authenticator.ID, 125, mfaSession,
	); !errors.Is(err, authentication.ErrAuthenticationTransactionInvalid) {
		t.Fatalf("TOTP transaction replay error = %v", err)
	}

	recoveryTransaction := authentication.AuthenticationTransaction{
		ID: identifier.MustNew().String(), Kind: "mfa_login", IdentityID: identity.ID,
		Requirement: []byte(`{"minimumLevel":"aal2"}`), State: []byte(`{}`),
		ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now,
	}
	if err := store.CreateAuthenticationTransaction(ctx, recoveryTransaction); err != nil {
		t.Fatal(err)
	}
	recoverySession := authentication.Session{
		ID: identifier.MustNew().String(), IdentityID: identity.ID,
		CreatedAt: now.Add(3 * time.Second), LastSeen: now.Add(3 * time.Second),
		ExpiresAt: now.Add(15 * time.Minute),
		Authentication: authentication.Recovery(
			authentication.Password(identifier.MustNew().String(), now),
			identifier.MustNew().String(), now.Add(3*time.Second),
		),
	}
	if err := store.CompleteRecoveryLogin(
		ctx, recoveryTransaction, codes[0].Digest, recoverySession,
	); err != nil {
		t.Fatalf("CompleteRecoveryLogin() error = %v", err)
	}
	storedRecovery, err := store.GetSession(ctx, recoverySession.ID)
	if err != nil || !storedRecovery.Authentication.Recovery {
		t.Fatalf("stored recovery session = %+v, %v", storedRecovery, err)
	}

	stepUpTransaction := authentication.AuthenticationTransaction{
		ID: identifier.MustNew().String(), Kind: "step_up", IdentityID: identity.ID,
		SessionID: mfaSession.ID, Audience: "commerce-api", Action: "order.refund",
		ResourceDigest: bytesOf(9, 32),
		Requirement:    []byte(`{"MinimumLevel":"aal2"}`), State: []byte(`{}`),
		ExpiresAt: now.Add(5 * time.Minute), CreatedAt: now,
	}
	if err := store.CreateAuthenticationTransaction(ctx, stepUpTransaction); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteTOTPTransaction(
		ctx, stepUpTransaction, authenticator.ID, 126,
		authentication.Session{
			ID: mfaSession.ID, IdentityID: identity.ID,
			LastSeen: now.Add(4 * time.Second),
			Authentication: authentication.MultiFactor(
				authentication.Password(identifier.MustNew().String(), now),
				identifier.MustNew().String(), now.Add(4*time.Second), authenticator.ID,
			),
		},
	); err != nil {
		t.Fatalf("CompleteTOTPTransaction() error = %v", err)
	}
	if err := store.CompleteTOTPTransaction(
		ctx, stepUpTransaction, authenticator.ID, 127,
		authentication.Session{
			ID: mfaSession.ID, IdentityID: identity.ID,
			Authentication: authentication.MultiFactor(
				authentication.Password(identifier.MustNew().String(), now),
				identifier.MustNew().String(), now.Add(5*time.Second), authenticator.ID,
			),
		},
	); !errors.Is(err, authentication.ErrAuthenticationTransactionInvalid) {
		t.Fatalf("step-up replay error = %v", err)
	}

	if err := store.RevokeTOTP(
		ctx, identity.ID, authenticator.ID, "test", now.Add(time.Minute),
	); err != nil {
		t.Fatalf("RevokeTOTP() error = %v", err)
	}
	if err := store.RevokeTOTP(
		ctx, identity.ID, authenticator.ID, "replay", now.Add(2*time.Minute),
	); !errors.Is(err, authentication.ErrTOTPNotFound) {
		t.Fatalf("revoke replay error = %v, want ErrTOTPNotFound", err)
	}
	required, err := db.GetValue(ctx, `
SELECT second_factor_required
FROM authentication_policies
WHERE identity_id = ?
`, identity.ID)
	if err != nil || required.Bool() {
		t.Fatalf("second_factor_required = %v, %v; want false", required, err)
	}
}

func TestPGPendingTOTPRequiresMatchingLiveSession(t *testing.T) {
	db := newDB(t)
	store := dao.NewPG(db)
	ctx := context.Background()

	identity, err := store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email:        "mfa-session-" + identifier.MustNew().String() + "@pg.test",
		PasswordHash: "password-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Model("identities").Ctx(ctx).Where("id", identity.ID).Delete()
	})

	now := time.Now().UTC()
	expiresAt := now.Add(time.Minute)
	err = store.CreatePendingTOTP(ctx, authentication.TOTPAuthenticator{
		ID: identifier.MustNew().String(), IdentityID: identity.ID,
		SecretCiphertext: make([]byte, 32), KeyVersion: 1,
		Algorithm: "SHA1", Digits: 6, PeriodSeconds: 30, Status: "pending",
		BindingSessionID: identifier.MustNew().String(), EnrollmentExpiresAt: &expiresAt,
		CreatedAt: now, UpdatedAt: now,
	})
	if !errors.Is(err, authentication.ErrTOTPEnrollmentInvalid) {
		t.Fatalf("CreatePendingTOTP() error = %v, want ErrTOTPEnrollmentInvalid", err)
	}
}

func bytesOf(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}
