package authentication

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var (
	ErrTOTPNotFound          = errors.New("TOTP authenticator not found")
	ErrTOTPEnrollmentInvalid = errors.New("TOTP enrollment invalid")
	ErrTOTPCodeInvalid       = errors.New("TOTP code invalid")
	ErrMFAUnavailable        = errors.New("multi-factor authentication unavailable")
)

type TOTPAuthenticator struct {
	ID                  string
	IdentityID          string
	Label               string
	SecretCiphertext    []byte
	KeyVersion          int
	Algorithm           string
	Digits              int
	PeriodSeconds       int
	Status              string
	BindingSessionID    string
	EnrollmentExpiresAt *time.Time
	FailedAttempts      int
	LastUsedStep        *int64
	CreatedAt           time.Time
	VerifiedAt          *time.Time
	UpdatedAt           time.Time
	LastUsedAt          *time.Time
}

type RecoveryCode struct {
	ID     string
	Digest []byte
}

type MFAStore interface {
	CountActiveTOTP(context.Context, string) (int, error)
	CreatePendingTOTP(context.Context, TOTPAuthenticator) error
	GetTOTP(context.Context, string, string) (TOTPAuthenticator, error)
	RecordTOTPFailure(context.Context, string, int) error
	ActivateTOTP(
		context.Context,
		TOTPAuthenticator,
		string,
		int64,
		string,
		[]RecoveryCode,
		time.Time,
	) error
	ListTOTP(context.Context, string) ([]TOTPAuthenticator, error)
	RevokeTOTP(context.Context, string, string, string, time.Time) error
	IsSecondFactorRequired(context.Context, string) (bool, error)
	CreateAuthenticationTransaction(context.Context, AuthenticationTransaction) error
	GetAuthenticationTransaction(context.Context, string) (AuthenticationTransaction, error)
	RecordAuthenticationTransactionFailure(context.Context, string, int) error
	CompleteTOTPLogin(
		context.Context,
		AuthenticationTransaction,
		string,
		int64,
		Session,
	) error
	CompleteRecoveryLogin(
		context.Context,
		AuthenticationTransaction,
		[]byte,
		Session,
	) error
	CompleteTOTPTransaction(
		context.Context,
		AuthenticationTransaction,
		string,
		int64,
		time.Time,
	) error
}

func (module *Module) ConfigureMFA(
	store MFAStore,
	verifier TOTPVerifier,
	secrets *SecretBox,
	recovery *RecoveryCodeCodec,
) error {
	if store == nil || verifier == nil || secrets == nil || recovery == nil {
		return ErrMFAUnavailable
	}
	module.mfa = store
	module.totp = verifier
	module.secrets = secrets
	module.recovery = recovery
	return nil
}

type BeginTOTPEnrollmentRequest struct {
	IdentityID  string
	SessionID   string
	AccountName string
	Label       string
}

type BeginTOTPEnrollmentResult struct {
	AuthenticatorID string
	URI             string
	Secret          string
	ExpiresAt       time.Time
}

func (module *Module) BeginTOTPEnrollment(
	ctx context.Context,
	request BeginTOTPEnrollmentRequest,
) (BeginTOTPEnrollmentResult, error) {
	if module.mfa == nil {
		return BeginTOTPEnrollmentResult{}, ErrMFAUnavailable
	}
	seed, err := module.totp.Generate(request.AccountName)
	if err != nil {
		return BeginTOTPEnrollmentResult{}, err
	}
	id := uuid.NewString()
	ciphertext, err := module.secrets.Seal(
		[]byte(seed.Secret), totpAdditionalData(request.IdentityID, id, 1),
	)
	if err != nil {
		return BeginTOTPEnrollmentResult{}, err
	}
	now := module.now().UTC()
	expiresAt := now.Add(module.cfg.CeremonyTTL)
	authenticator := TOTPAuthenticator{
		ID: id, IdentityID: request.IdentityID, Label: request.Label,
		SecretCiphertext: ciphertext, KeyVersion: 1,
		Algorithm: seed.Algorithm, Digits: seed.Digits, PeriodSeconds: seed.Period,
		Status: "pending", BindingSessionID: request.SessionID,
		EnrollmentExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := module.mfa.CreatePendingTOTP(ctx, authenticator); err != nil {
		return BeginTOTPEnrollmentResult{}, err
	}
	return BeginTOTPEnrollmentResult{
		AuthenticatorID: id, URI: seed.URI, Secret: seed.Secret, ExpiresAt: expiresAt,
	}, nil
}

type FinishTOTPEnrollmentRequest struct {
	IdentityID      string
	SessionID       string
	AuthenticatorID string
	Code            string
}

type FinishTOTPEnrollmentResult struct {
	Authenticator TOTPAuthenticator
	RecoveryCodes []string
}

func (module *Module) FinishTOTPEnrollment(
	ctx context.Context,
	request FinishTOTPEnrollmentRequest,
) (FinishTOTPEnrollmentResult, error) {
	if module.mfa == nil {
		return FinishTOTPEnrollmentResult{}, ErrMFAUnavailable
	}
	authenticator, err := module.mfa.GetTOTP(
		ctx, request.IdentityID, request.AuthenticatorID,
	)
	if err != nil {
		return FinishTOTPEnrollmentResult{}, ErrTOTPEnrollmentInvalid
	}
	now := module.now().UTC()
	if authenticator.Status != "pending" ||
		authenticator.BindingSessionID != request.SessionID ||
		authenticator.EnrollmentExpiresAt == nil ||
		!authenticator.EnrollmentExpiresAt.After(now) ||
		authenticator.FailedAttempts >= 5 {
		return FinishTOTPEnrollmentResult{}, ErrTOTPEnrollmentInvalid
	}
	secret, err := module.secrets.Open(
		authenticator.SecretCiphertext,
		totpAdditionalData(authenticator.IdentityID, authenticator.ID, authenticator.KeyVersion),
	)
	if err != nil {
		return FinishTOTPEnrollmentResult{}, ErrTOTPEnrollmentInvalid
	}
	step, valid, err := module.totp.Verify(string(secret), request.Code, now, nil)
	if err != nil {
		return FinishTOTPEnrollmentResult{}, err
	}
	if !valid {
		_ = module.mfa.RecordTOTPFailure(ctx, authenticator.ID, 5)
		return FinishTOTPEnrollmentResult{}, ErrTOTPCodeInvalid
	}
	plaintextCodes, digests, err := module.recovery.Generate()
	if err != nil {
		return FinishTOTPEnrollmentResult{}, err
	}
	recoveryCodes := make([]RecoveryCode, len(digests))
	for index := range digests {
		recoveryCodes[index] = RecoveryCode{ID: uuid.NewString(), Digest: digests[index]}
	}
	setID := uuid.NewString()
	if err := module.mfa.ActivateTOTP(
		ctx, authenticator, request.SessionID, step,
		setID, recoveryCodes, now,
	); err != nil {
		return FinishTOTPEnrollmentResult{}, err
	}
	authenticator.Status = "active"
	authenticator.BindingSessionID = ""
	authenticator.EnrollmentExpiresAt = nil
	authenticator.LastUsedStep = &step
	authenticator.VerifiedAt = &now
	authenticator.UpdatedAt = now
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventTOTPEnrolled, IdentityID: request.IdentityID,
		CredentialID: authenticator.ID, Label: authenticator.Label, OccurredAt: now,
		Detail: map[string]any{"recoveryCodeCount": len(recoveryCodes)},
	})
	return FinishTOTPEnrollmentResult{
		Authenticator: authenticator, RecoveryCodes: plaintextCodes,
	}, nil
}

func (module *Module) ListTOTP(
	ctx context.Context,
	identityID string,
) ([]TOTPAuthenticator, error) {
	if module.mfa == nil {
		return nil, ErrMFAUnavailable
	}
	return module.mfa.ListTOTP(ctx, identityID)
}

func (module *Module) RevokeTOTP(
	ctx context.Context,
	identityID, authenticatorID string,
) error {
	if module.mfa == nil {
		return ErrMFAUnavailable
	}
	now := module.now().UTC()
	if err := module.mfa.RevokeTOTP(
		ctx, identityID, authenticatorID, "removed by account owner", now,
	); err != nil {
		return err
	}
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventTOTPRevoked, IdentityID: identityID,
		CredentialID: authenticatorID, OccurredAt: now,
	})
	return nil
}

func totpAdditionalData(identityID, authenticatorID string, keyVersion int) []byte {
	return []byte(
		identityID + "\x00" + authenticatorID + "\x00" + strconv.Itoa(keyVersion),
	)
}
