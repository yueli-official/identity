package authentication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

type mfaTestStore struct {
	authenticator TOTPAuthenticator
	recovery      []RecoveryCode
	failures      int
}

func (*mfaTestStore) CountActiveTOTP(context.Context, string) (int, error) { return 0, nil }
func (store *mfaTestStore) CreatePendingTOTP(
	_ context.Context,
	authenticator TOTPAuthenticator,
) error {
	store.authenticator = authenticator
	return nil
}
func (store *mfaTestStore) GetTOTP(
	_ context.Context,
	identityID, id string,
) (TOTPAuthenticator, error) {
	if store.authenticator.IdentityID != identityID || store.authenticator.ID != id {
		return TOTPAuthenticator{}, ErrTOTPNotFound
	}
	return store.authenticator, nil
}
func (store *mfaTestStore) RecordTOTPFailure(context.Context, string, int) error {
	store.failures++
	store.authenticator.FailedAttempts++
	return nil
}
func (store *mfaTestStore) ActivateTOTP(
	_ context.Context,
	authenticator TOTPAuthenticator,
	sessionID string,
	step int64,
	_ string,
	recovery []RecoveryCode,
	now time.Time,
) error {
	if authenticator.BindingSessionID != sessionID {
		return ErrTOTPEnrollmentInvalid
	}
	store.authenticator = authenticator
	store.authenticator.Status = "active"
	store.authenticator.BindingSessionID = ""
	store.authenticator.EnrollmentExpiresAt = nil
	store.authenticator.LastUsedStep = &step
	store.authenticator.VerifiedAt = &now
	store.recovery = recovery
	return nil
}
func (*mfaTestStore) ListTOTP(context.Context, string) ([]TOTPAuthenticator, error) {
	return nil, nil
}
func (*mfaTestStore) RevokeTOTP(context.Context, string, string, string, time.Time) error {
	return nil
}

func TestTOTPEnrollmentBindsSessionEncryptsSecretAndReturnsRecoveryCodesOnce(t *testing.T) {
	passkeys := newPasskeyTestStore()
	module, err := NewModule(
		passkeys, passkeys, &passkeyTestVerifier{store: passkeys},
		ModuleConfig{CeremonyTTL: 5 * time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}
	totpVerifier, _ := NewTOTPVerifier("Yueli Account")
	box, _ := NewSecretBox([]byte("test-master-secret-at-least-thirty-two-bytes"))
	recovery, _ := NewRecoveryCodeCodec([]byte("test-master-secret-at-least-thirty-two-bytes"))
	store := &mfaTestStore{}
	if err := module.ConfigureMFA(store, totpVerifier, box, recovery); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 5, 0, time.UTC)
	module.now = func() time.Time { return now }

	begin, err := module.BeginTOTPEnrollment(context.Background(), BeginTOTPEnrollmentRequest{
		IdentityID: "identity-1", SessionID: "session-1",
		AccountName: "user@example.test", Label: "Authenticator",
	})
	if err != nil {
		t.Fatal(err)
	}
	if begin.Secret == "" || begin.URI == "" ||
		string(store.authenticator.SecretCiphertext) == begin.Secret {
		t.Fatalf("begin = %+v, stored = %+v", begin, store.authenticator)
	}
	code, err := totp.GenerateCode(begin.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = module.FinishTOTPEnrollment(context.Background(), FinishTOTPEnrollmentRequest{
		IdentityID: "identity-1", SessionID: "wrong-session",
		AuthenticatorID: begin.AuthenticatorID, Code: code,
	})
	if !errors.Is(err, ErrTOTPEnrollmentInvalid) {
		t.Fatalf("wrong-session error = %v", err)
	}
	_, err = module.FinishTOTPEnrollment(context.Background(), FinishTOTPEnrollmentRequest{
		IdentityID: "identity-1", SessionID: "session-1",
		AuthenticatorID: begin.AuthenticatorID, Code: "000000",
	})
	if !errors.Is(err, ErrTOTPCodeInvalid) || store.failures != 1 {
		t.Fatalf("invalid-code error = %v, failures = %d", err, store.failures)
	}
	result, err := module.FinishTOTPEnrollment(context.Background(), FinishTOTPEnrollmentRequest{
		IdentityID: "identity-1", SessionID: "session-1",
		AuthenticatorID: begin.AuthenticatorID, Code: code,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authenticator.Status != "active" ||
		len(result.RecoveryCodes) != recoveryCodeCount ||
		len(store.recovery) != recoveryCodeCount {
		t.Fatalf("finish = %+v, stored recovery = %d", result, len(store.recovery))
	}
}

var _ MFAStore = (*mfaTestStore)(nil)
