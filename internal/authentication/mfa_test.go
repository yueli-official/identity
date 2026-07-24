package authentication

import (
	"context"
	"crypto/hmac"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

type mfaTestStore struct {
	authenticator TOTPAuthenticator
	recovery      []RecoveryCode
	failures      int
	required      bool
	transaction   AuthenticationTransaction
	session       Session
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
func (store *mfaTestStore) ListTOTP(_ context.Context, identityID string) ([]TOTPAuthenticator, error) {
	if store.authenticator.IdentityID == identityID && store.authenticator.Status == "active" {
		return []TOTPAuthenticator{store.authenticator}, nil
	}
	return []TOTPAuthenticator{}, nil
}
func (*mfaTestStore) RevokeTOTP(context.Context, string, string, string, time.Time) error {
	return nil
}
func (store *mfaTestStore) IsSecondFactorRequired(context.Context, string) (bool, error) {
	return store.required, nil
}
func (store *mfaTestStore) CreateAuthenticationTransaction(
	_ context.Context,
	transaction AuthenticationTransaction,
) error {
	store.transaction = transaction
	return nil
}
func (store *mfaTestStore) GetAuthenticationTransaction(
	_ context.Context,
	id string,
) (AuthenticationTransaction, error) {
	if store.transaction.ID != id {
		return AuthenticationTransaction{}, ErrAuthenticationTransactionInvalid
	}
	return store.transaction, nil
}
func (store *mfaTestStore) RecordAuthenticationTransactionFailure(
	context.Context, string, int,
) error {
	store.transaction.FailedAttempts++
	return nil
}
func (store *mfaTestStore) CompleteTOTPLogin(
	_ context.Context,
	transaction AuthenticationTransaction,
	authenticatorID string,
	step int64,
	session Session,
) error {
	if store.transaction.ConsumedAt != nil ||
		store.transaction.ID != transaction.ID ||
		store.authenticator.ID != authenticatorID {
		return ErrAuthenticationTransactionInvalid
	}
	now := session.Authentication.AuthenticatedAt
	store.transaction.ConsumedAt = &now
	store.authenticator.LastUsedStep = &step
	store.authenticator.LastUsedAt = &now
	store.session = session
	return nil
}
func (store *mfaTestStore) CompleteRecoveryLogin(
	_ context.Context,
	transaction AuthenticationTransaction,
	digest []byte,
	session Session,
) error {
	if store.transaction.ConsumedAt != nil || store.transaction.ID != transaction.ID {
		return ErrAuthenticationTransactionInvalid
	}
	for index := range store.recovery {
		if hmac.Equal(store.recovery[index].Digest, digest) {
			now := session.Authentication.AuthenticatedAt
			store.transaction.ConsumedAt = &now
			store.recovery = append(store.recovery[:index], store.recovery[index+1:]...)
			store.session = session
			return nil
		}
	}
	return ErrRecoveryCodeInvalid
}
func (store *mfaTestStore) CompleteTOTPTransaction(
	_ context.Context,
	transaction AuthenticationTransaction,
	authenticatorID string,
	step int64,
	session Session,
) error {
	if store.transaction.ConsumedAt != nil ||
		store.transaction.ID != transaction.ID ||
		store.authenticator.ID != authenticatorID {
		return ErrAuthenticationTransactionInvalid
	}
	now := session.Authentication.AuthenticatedAt
	store.transaction.ConsumedAt = &now
	store.authenticator.LastUsedStep = &step
	store.authenticator.LastUsedAt = &now
	store.session = session
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

func TestTOTPLoginCreatesAAL2SessionAndConsumesTransaction(t *testing.T) {
	passkeys := newPasskeyTestStore()
	module, err := NewModule(
		passkeys, passkeys, &passkeyTestVerifier{store: passkeys},
		ModuleConfig{SessionTTL: time.Hour, TransactionTTL: 5 * time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}
	totpVerifier, _ := NewTOTPVerifier("Yueli Account")
	box, _ := NewSecretBox([]byte("test-master-secret-at-least-thirty-two-bytes"))
	recovery, _ := NewRecoveryCodeCodec([]byte("test-master-secret-at-least-thirty-two-bytes"))
	store := &mfaTestStore{required: true}
	if err := module.ConfigureMFA(store, totpVerifier, box, recovery); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 5, 0, time.UTC)
	module.now = func() time.Time { return now }
	seed, err := totpVerifier.Generate("user@example.test")
	if err != nil {
		t.Fatal(err)
	}
	authenticatorID := "totp-1"
	ciphertext, err := box.Seal(
		[]byte(seed.Secret), totpAdditionalData("identity-1", authenticatorID, 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	store.authenticator = TOTPAuthenticator{
		ID: authenticatorID, IdentityID: "identity-1", Status: "active",
		SecretCiphertext: ciphertext, KeyVersion: 1,
	}

	begin, err := module.BeginSecondFactor(
		context.Background(), "identity-1",
		Password("primary-event", now.Add(-time.Second)),
		"Test Browser", "192.0.2.10",
	)
	if err != nil || !begin.Required || begin.TransactionID == "" {
		t.Fatalf("BeginSecondFactor() = %+v, %v", begin, err)
	}
	code, err := totp.GenerateCode(seed.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := module.FinishTOTPLogin(context.Background(), FinishTOTPLoginRequest{
		TransactionID: begin.TransactionID, Code: code,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Authentication.Level != LevelAAL2 ||
		result.Authentication.Profile != ProfileMultiFactor ||
		len(result.Authentication.Methods) != 2 ||
		store.session.UserAgent != "Test Browser" {
		t.Fatalf("authentication result = %+v, session = %+v", result, store.session)
	}
	if _, err := module.FinishTOTPLogin(context.Background(), FinishTOTPLoginRequest{
		TransactionID: begin.TransactionID, Code: code,
	}); !errors.Is(err, ErrAuthenticationTransactionInvalid) {
		t.Fatalf("transaction replay error = %v", err)
	}

	recoveryPlaintext, recoveryDigests, err := recovery.Generate()
	if err != nil {
		t.Fatal(err)
	}
	store.recovery = []RecoveryCode{{ID: "recovery-1", Digest: recoveryDigests[0]}}
	recoveryBegin, err := module.BeginSecondFactor(
		context.Background(), "identity-1",
		Password("primary-event-2", now), "Test Browser", "192.0.2.10",
	)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := module.FinishRecoveryLogin(
		context.Background(), FinishRecoveryLoginRequest{
			TransactionID: recoveryBegin.TransactionID, Code: recoveryPlaintext[0],
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Authentication.Recovery ||
		recovered.Authentication.Level != LevelAAL1 ||
		len(store.recovery) != 0 {
		t.Fatalf("recovery authentication = %+v, remaining = %d",
			recovered.Authentication, len(store.recovery))
	}

	now = now.Add(30 * time.Second)
	baseContext := Password("step-up-primary", now.Add(-time.Minute))
	stepUp, err := module.BeginStepUp(context.Background(), BeginStepUpRequest{
		IdentityID: "identity-1", SessionID: "session-existing",
		Audience: "commerce-api", Action: "order.refund", Resource: "order:123",
		Requirement: Requirement{
			FreshWithin: 5 * time.Minute, MinimumLevel: LevelAAL2,
			MinimumProfile: ProfileMultiFactor, MinimumFactorCount: 2,
		},
		Context: baseContext,
	})
	if err != nil || stepUp.TransactionID == "" || stepUp.Satisfied {
		t.Fatalf("BeginStepUp() = %+v, %v", stepUp, err)
	}
	stepUpCode, err := totp.GenerateCode(seed.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := module.FinishTOTPAction(context.Background(), FinishTOTPActionRequest{
		TransactionID: stepUp.TransactionID,
		Session: Session{
			ID: "session-existing", IdentityID: "identity-1",
			Authentication: baseContext,
		},
		Code: stepUpCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.Audience != "commerce-api" || proof.Action != "order.refund" ||
		len(proof.ResourceDigest) != 32 || proof.Authentication.Level != LevelAAL2 {
		t.Fatalf("step-up proof material = %+v", proof)
	}
	if _, err := module.FinishTOTPAction(context.Background(), FinishTOTPActionRequest{
		TransactionID: stepUp.TransactionID,
		Session: Session{
			ID: "session-existing", IdentityID: "identity-1",
			Authentication: baseContext,
		},
		Code: stepUpCode,
	}); !errors.Is(err, ErrAuthenticationTransactionInvalid) {
		t.Fatalf("step-up replay error = %v", err)
	}
}

var _ MFAStore = (*mfaTestStore)(nil)
