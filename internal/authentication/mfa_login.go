package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yueli-official/foundation/go/identifier"
)

var ErrAuthenticationTransactionInvalid = errors.New("authentication transaction invalid")
var ErrRecoveryCodeInvalid = errors.New("recovery code invalid")

type AuthenticationTransaction struct {
	ID             string
	Kind           string
	IdentityID     string
	SessionID      string
	Audience       string
	Action         string
	ResourceDigest []byte
	Requirement    json.RawMessage
	State          json.RawMessage
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	FailedAttempts int
	CreatedAt      time.Time
}

type loginTransactionState struct {
	Primary   Context `json:"primary"`
	UserAgent string  `json:"userAgent"`
	IP        string  `json:"ip"`
}

type BeginSecondFactorResult struct {
	Required      bool
	TransactionID string
	ExpiresAt     time.Time
	Methods       []string
}

func (module *Module) BeginSecondFactor(
	ctx context.Context,
	identityID string,
	primary Context,
	userAgent, ip string,
) (BeginSecondFactorResult, error) {
	if module.mfa == nil {
		return BeginSecondFactorResult{}, nil
	}
	required, err := module.mfa.IsSecondFactorRequired(ctx, identityID)
	if err != nil {
		return BeginSecondFactorResult{}, err
	}
	if !required {
		return BeginSecondFactorResult{}, nil
	}
	state, err := json.Marshal(loginTransactionState{
		Primary: primary, UserAgent: userAgent, IP: ip,
	})
	if err != nil {
		return BeginSecondFactorResult{}, err
	}
	now := module.now().UTC()
	transaction := AuthenticationTransaction{
		ID: identifier.MustNew().String(), Kind: "mfa_login", IdentityID: identityID,
		Requirement: json.RawMessage(`{"minimumLevel":"aal2"}`),
		State:       state, ExpiresAt: now.Add(module.cfg.TransactionTTL), CreatedAt: now,
	}
	if err := module.mfa.CreateAuthenticationTransaction(ctx, transaction); err != nil {
		return BeginSecondFactorResult{}, err
	}
	return BeginSecondFactorResult{
		Required: true, TransactionID: transaction.ID,
		ExpiresAt: transaction.ExpiresAt, Methods: []string{"totp", "recovery_code"},
	}, nil
}

type FinishRecoveryLoginRequest struct {
	TransactionID string
	Code          string
}

func (module *Module) FinishRecoveryLogin(
	ctx context.Context,
	request FinishRecoveryLoginRequest,
) (AuthenticationResult, error) {
	if module.mfa == nil {
		return AuthenticationResult{}, ErrMFAUnavailable
	}
	transaction, err := module.mfa.GetAuthenticationTransaction(ctx, request.TransactionID)
	if err != nil {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	now := module.now().UTC()
	if transaction.Kind != "mfa_login" || transaction.ConsumedAt != nil ||
		!transaction.ExpiresAt.After(now) || transaction.FailedAttempts >= 5 {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	var state loginTransactionState
	if err := json.Unmarshal(transaction.State, &state); err != nil {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	auth := Recovery(state.Primary, identifier.MustNew().String(), now)
	session := Session{
		ID: identifier.MustNew().String(), IdentityID: transaction.IdentityID,
		CreatedAt: now, LastSeen: now, UserAgent: state.UserAgent, IP: state.IP,
		ExpiresAt: now.Add(module.cfg.RecoveryTTL), Authentication: auth,
	}
	if err := module.mfa.CompleteRecoveryLogin(
		ctx, transaction, module.recovery.Digest(request.Code), session,
	); err != nil {
		if errors.Is(err, ErrRecoveryCodeInvalid) {
			_ = module.mfa.RecordAuthenticationTransactionFailure(ctx, transaction.ID, 5)
		}
		return AuthenticationResult{}, err
	}
	if module.cache != nil {
		_ = module.cache.CreateSession(ctx, session, module.cfg.RecoveryTTL)
	}
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventRecoveryUsed, IdentityID: transaction.IdentityID,
		SessionID: session.ID, OccurredAt: now,
	})
	return AuthenticationResult{
		SessionID: session.ID, IdentityID: transaction.IdentityID,
		Authentication: auth,
	}, nil
}

type FinishTOTPLoginRequest struct {
	TransactionID string
	Code          string
}

func (module *Module) FinishTOTPLogin(
	ctx context.Context,
	request FinishTOTPLoginRequest,
) (AuthenticationResult, error) {
	if module.mfa == nil {
		return AuthenticationResult{}, ErrMFAUnavailable
	}
	transaction, err := module.mfa.GetAuthenticationTransaction(ctx, request.TransactionID)
	if err != nil {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	now := module.now().UTC()
	if transaction.Kind != "mfa_login" || transaction.ConsumedAt != nil ||
		!transaction.ExpiresAt.After(now) || transaction.FailedAttempts >= 5 {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	var state loginTransactionState
	if err := json.Unmarshal(transaction.State, &state); err != nil {
		return AuthenticationResult{}, ErrAuthenticationTransactionInvalid
	}
	authenticators, err := module.mfa.ListTOTP(ctx, transaction.IdentityID)
	if err != nil {
		return AuthenticationResult{}, err
	}
	var matched *TOTPAuthenticator
	var matchedStep int64
	for index := range authenticators {
		authenticator := &authenticators[index]
		if authenticator.Status != "active" {
			continue
		}
		secret, openErr := module.secrets.Open(
			authenticator.SecretCiphertext,
			totpAdditionalData(
				authenticator.IdentityID, authenticator.ID, authenticator.KeyVersion,
			),
		)
		if openErr != nil {
			continue
		}
		step, valid, verifyErr := module.totp.Verify(
			string(secret), request.Code, now, authenticator.LastUsedStep,
		)
		if verifyErr != nil {
			return AuthenticationResult{}, verifyErr
		}
		if valid {
			matched, matchedStep = authenticator, step
			break
		}
	}
	if matched == nil {
		_ = module.mfa.RecordAuthenticationTransactionFailure(ctx, transaction.ID, 5)
		return AuthenticationResult{}, ErrTOTPCodeInvalid
	}
	auth := MultiFactor(state.Primary, identifier.MustNew().String(), now, matched.ID)
	session := Session{
		ID: identifier.MustNew().String(), IdentityID: transaction.IdentityID,
		CreatedAt: now, LastSeen: now, UserAgent: state.UserAgent, IP: state.IP,
		ExpiresAt: now.Add(module.cfg.SessionTTL), Authentication: auth,
	}
	if err := module.mfa.CompleteTOTPLogin(
		ctx, transaction, matched.ID, matchedStep, session,
	); err != nil {
		return AuthenticationResult{}, err
	}
	if module.cache != nil {
		_ = module.cache.CreateSession(ctx, session, module.cfg.SessionTTL)
	}
	module.recordEvent(ctx, SecurityEvent{
		Kind: EventTOTPLogin, IdentityID: transaction.IdentityID,
		CredentialID: matched.ID, SessionID: session.ID, OccurredAt: now,
	})
	return AuthenticationResult{
		SessionID: session.ID, IdentityID: transaction.IdentityID,
		Authentication: auth,
	}, nil
}
