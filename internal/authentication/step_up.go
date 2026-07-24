package authentication

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrStepUpRequestInvalid    = errors.New("step-up request invalid")
	ErrStepUpMethodUnavailable = errors.New("step-up method unavailable")
)

type BeginStepUpRequest struct {
	IdentityID  string
	SessionID   string
	Audience    string
	Action      string
	Resource    string
	Requirement Requirement
	Context     Context
}

type StepUpProofMaterial struct {
	ID             string
	IdentityID     string
	SessionID      string
	Audience       string
	Action         string
	ResourceDigest []byte
	Authentication Context
	IssuedAt       time.Time
}

type BeginStepUpResult struct {
	Satisfied     bool
	TransactionID string
	ExpiresAt     time.Time
	Methods       []string
	Proof         StepUpProofMaterial
}

func (module *Module) BeginStepUp(
	ctx context.Context,
	request BeginStepUpRequest,
) (BeginStepUpResult, error) {
	audience := strings.TrimSpace(request.Audience)
	action := strings.TrimSpace(request.Action)
	resource := strings.TrimSpace(request.Resource)
	if module.mfa == nil || request.IdentityID == "" || request.SessionID == "" ||
		audience == "" || action == "" || resource == "" ||
		len(audience) > 200 || len(action) > 200 || len(resource) > 2048 {
		return BeginStepUpResult{}, ErrStepUpRequestInvalid
	}
	now := module.now().UTC()
	digest := sha256.Sum256([]byte(resource))
	if decision := Evaluate(request.Context, request.Requirement, now); decision.Satisfied {
		return BeginStepUpResult{
			Satisfied: true,
			Proof: StepUpProofMaterial{
				ID: uuid.NewString(), IdentityID: request.IdentityID,
				SessionID: request.SessionID, Audience: audience, Action: action,
				ResourceDigest: digest[:], Authentication: request.Context, IssuedAt: now,
			},
		}, nil
	}
	if request.Requirement.PhishingResistant ||
		request.Requirement.UserVerification ||
		profileRank(request.Requirement.MinimumProfile) > profileRank(ProfileMultiFactor) ||
		levelRank(request.Requirement.MinimumLevel) > levelRank(LevelAAL2) {
		return BeginStepUpResult{}, ErrStepUpMethodUnavailable
	}
	requirement, err := json.Marshal(request.Requirement)
	if err != nil {
		return BeginStepUpResult{}, err
	}
	transaction := AuthenticationTransaction{
		ID: uuid.NewString(), Kind: "step_up", IdentityID: request.IdentityID,
		SessionID: request.SessionID, Audience: audience, Action: action,
		ResourceDigest: digest[:], Requirement: requirement, State: json.RawMessage(`{}`),
		ExpiresAt: now.Add(module.cfg.TransactionTTL), CreatedAt: now,
	}
	if err := module.mfa.CreateAuthenticationTransaction(ctx, transaction); err != nil {
		return BeginStepUpResult{}, err
	}
	return BeginStepUpResult{
		TransactionID: transaction.ID, ExpiresAt: transaction.ExpiresAt,
		Methods: []string{"totp"},
	}, nil
}

type FinishTOTPActionRequest struct {
	TransactionID string
	Session       Session
	Code          string
}

func (module *Module) FinishTOTPAction(
	ctx context.Context,
	request FinishTOTPActionRequest,
) (StepUpProofMaterial, error) {
	if module.mfa == nil {
		return StepUpProofMaterial{}, ErrMFAUnavailable
	}
	transaction, err := module.mfa.GetAuthenticationTransaction(ctx, request.TransactionID)
	if err != nil {
		return StepUpProofMaterial{}, ErrAuthenticationTransactionInvalid
	}
	now := module.now().UTC()
	if transaction.Kind != "step_up" || transaction.SessionID != request.Session.ID ||
		transaction.ConsumedAt != nil || !transaction.ExpiresAt.After(now) ||
		transaction.FailedAttempts >= 5 {
		return StepUpProofMaterial{}, ErrAuthenticationTransactionInvalid
	}
	var requirement Requirement
	if err := json.Unmarshal(transaction.Requirement, &requirement); err != nil {
		return StepUpProofMaterial{}, ErrAuthenticationTransactionInvalid
	}
	authenticators, err := module.mfa.ListTOTP(ctx, transaction.IdentityID)
	if err != nil {
		return StepUpProofMaterial{}, err
	}
	var matched *TOTPAuthenticator
	var matchedStep int64
	for index := range authenticators {
		authenticator := &authenticators[index]
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
			return StepUpProofMaterial{}, verifyErr
		}
		if valid {
			matched, matchedStep = authenticator, step
			break
		}
	}
	if matched == nil {
		_ = module.mfa.RecordAuthenticationTransactionFailure(ctx, transaction.ID, 5)
		return StepUpProofMaterial{}, ErrTOTPCodeInvalid
	}
	elevated := MultiFactor(request.Session.Authentication, uuid.NewString(), now, matched.ID)
	if !Evaluate(elevated, requirement, now).Satisfied {
		return StepUpProofMaterial{}, ErrStepUpMethodUnavailable
	}
	elevatedSession := request.Session
	elevatedSession.Authentication = elevated
	elevatedSession.LastSeen = now
	if err := module.mfa.CompleteTOTPTransaction(
		ctx, transaction, matched.ID, matchedStep, elevatedSession,
	); err != nil {
		return StepUpProofMaterial{}, err
	}
	if module.cache != nil {
		remaining := elevatedSession.ExpiresAt.Sub(now)
		if remaining > 0 {
			_ = module.cache.CreateSession(ctx, elevatedSession, remaining)
		}
	}
	return StepUpProofMaterial{
		ID: uuid.NewString(), IdentityID: transaction.IdentityID,
		SessionID: transaction.SessionID, Audience: transaction.Audience,
		Action: transaction.Action, ResourceDigest: transaction.ResourceDigest,
		Authentication: elevated, IssuedAt: now,
	}, nil
}
