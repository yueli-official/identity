package logic

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/yueli-official/foundation/go/abuse"

	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/identityabuse"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	identitypassword "platform/services/identity/internal/password"
	"platform/services/identity/internal/repo"
)

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
	AttemptID string
	Proof     string
}

type LoginOutput struct {
	SessionID      string
	Identity       model.Identity
	MFARequired    bool
	MFATransaction string
	MFAExpiresAt   time.Time
	MFAMethods     []string
}

func (s *Service) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	email := CanonicalizeEmail(in.Email)
	var admission abuse.Admission
	gated := in.AttemptID != "" || in.IP != ""
	if gated {
		attemptID := in.AttemptID
		if attemptID == "" {
			attemptID = uuid.NewString()
		}
		network, err := identityabuse.NetworkPrefix(in.IP)
		if err != nil {
			return LoginOutput{}, iderr.AbuseUnavailable()
		}
		admission, err = identityabuse.Admit(
			ctx, s.abuse.Login, attemptID, network, email, in.Proof,
		)
		if err != nil {
			if abuse.IsKind(err, abuse.ErrorConflict) {
				return LoginOutput{}, iderr.AbuseAttemptReplayed()
			}
			return LoginOutput{}, iderr.AbuseUnavailable()
		}
		switch admission.Disposition {
		case abuse.DispositionAllow:
			if admission.Replay {
				return LoginOutput{}, iderr.AbuseAttemptReplayed()
			}
		case abuse.DispositionChallenge:
			return LoginOutput{}, iderr.ChallengeRequired(attemptID)
		default:
			return LoginOutput{}, iderr.AccountLocked()
		}
	}

	fail := func(reason string) (LoginOutput, error) {
		if gated {
			if err := s.abuse.Login.Resolve(ctx, admission.Receipt, "credentials_rejected"); err != nil {
				return LoginOutput{}, iderr.AbuseUnavailable()
			}
		}
		s.audit(ctx, AuditEvent{
			Event:  EvLoginFailure,
			Email:  email,
			Result: "failure",
			Detail: map[string]any{"reason": reason},
		})
		return LoginOutput{}, iderr.InvalidCredentials()
	}

	id, err := s.store.GetByEmail(ctx, email)
	if errors.Is(err, repo.ErrIdentityMissing) {
		VerifyDummy(in.Password) // equalize timing vs the wrong-password path
		return fail("unknown_email")
	}
	if err != nil {
		return LoginOutput{}, err
	}
	hash, err := s.store.GetPasswordHash(ctx, id.ID)
	if err != nil || !s.passwords.Verify(hash, in.Password) {
		return fail("bad_password")
	}
	if id.Status == model.StatusDisabled {
		return LoginOutput{}, iderr.AccountDisabled()
	}
	if id.Status == model.StatusDeleted {
		return fail("deleted")
	}
	// Successful verification is the migration boundary for legacy bcrypt or
	// older Argon2id parameters. A rehash failure must not turn valid
	// credentials into a login outage.
	if s.passwords.NeedsRehash(hash) {
		if upgraded, hashErr := s.passwords.Hash(
			identitypassword.Normalize(in.Password),
		); hashErr == nil {
			_ = s.store.UpdatePasswordHash(ctx, id.ID, upgraded)
		}
	}

	if gated {
		if err := s.abuse.Login.Resolve(ctx, admission.Receipt, "authenticated"); err != nil {
			return LoginOutput{}, iderr.AbuseUnavailable()
		}
	}
	// Success: mint a fresh (rotated) session id after Abuse resolution.
	authenticatedAt := s.now().UTC()
	primary := authentication.Password(uuid.NewString(), authenticatedAt)
	if s.secondFactor != nil {
		secondFactor, err := s.secondFactor.BeginSecondFactor(
			ctx, id.ID, primary, in.UserAgent, in.IP,
		)
		if err != nil {
			return LoginOutput{}, err
		}
		if secondFactor.Required {
			return LoginOutput{
				Identity: id, MFARequired: true,
				MFATransaction: secondFactor.TransactionID,
				MFAExpiresAt:   secondFactor.ExpiresAt,
				MFAMethods:     secondFactor.Methods,
			}, nil
		}
	}
	sess := model.Session{
		ID: uuid.NewString(), IdentityID: id.ID,
		CreatedAt: authenticatedAt, LastSeen: authenticatedAt, UserAgent: in.UserAgent, IP: in.IP,
		Authentication: primary,
	}
	if err := s.store.CreateSession(ctx, sess, s.cfg.SessionIdleTTL); err != nil {
		return LoginOutput{}, err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvLoginSuccess,
		ActorID:  id.ID,
		TargetID: id.ID,
		Email:    id.Email,
	})
	return LoginOutput{SessionID: sess.ID, Identity: id}, nil
}
