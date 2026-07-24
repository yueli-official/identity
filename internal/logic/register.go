package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/yueli-official/foundation/go/abuse"
	"platform/services/identity/internal/identityabuse"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	identitypassword "platform/services/identity/internal/password"
	"platform/services/identity/internal/repo"
)

type RegisterInput struct {
	ID          string // optional trusted seed/bootstrap sub; empty → generated
	Email       string
	Password    string
	DisplayName string
	Locale      string
	AttemptID   string
	IP          string
	Proof       string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (model.Identity, error) {
	email := CanonicalizeEmail(in.Email)
	if in.AttemptID != "" || in.IP != "" {
		attemptID := in.AttemptID
		if attemptID == "" {
			attemptID = uuid.NewString()
		}
		network, err := identityabuse.NetworkPrefix(in.IP)
		if err != nil {
			return model.Identity{}, iderr.AbuseUnavailable()
		}
		admission, err := identityabuse.Admit(
			ctx, s.abuse.Register, attemptID, network, email, in.Proof,
		)
		if err != nil {
			if abuse.IsKind(err, abuse.ErrorConflict) {
				return model.Identity{}, iderr.AbuseAttemptReplayed()
			}
			return model.Identity{}, iderr.AbuseUnavailable()
		}
		switch admission.Disposition {
		case abuse.DispositionAllow:
			if admission.Replay {
				return model.Identity{}, iderr.AbuseAttemptReplayed()
			}
		case abuse.DispositionChallenge:
			return model.Identity{}, iderr.ChallengeRequired(attemptID)
		default:
			return model.Identity{}, iderr.AccountLocked()
		}
	}
	if err := ValidateEmail(email); err != nil {
		return model.Identity{}, iderr.InvalidEmail(email)
	}
	normalized, err := s.preparePassword(ctx, in.Password, identitypassword.Context{
		Email: email, DisplayName: in.DisplayName,
	})
	if err != nil {
		return model.Identity{}, err
	}
	hash, err := s.hashPassword(normalized)
	if err != nil {
		return model.Identity{}, err
	}
	id, err := s.store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		ID: in.ID, Email: email, DisplayName: in.DisplayName, Locale: in.Locale, PasswordHash: hash,
	})
	if errors.Is(err, repo.ErrEmailTaken) {
		return model.Identity{}, iderr.EmailTaken(email)
	}
	if err != nil {
		return model.Identity{}, err
	}
	_ = s.store.GrantRole(ctx, id.ID, DefaultRole) // best-effort default role
	// Audit order mirrors the causal order: the identity must exist before a role
	// can be granted, so identity.register is emitted first, then role.default_granted.
	s.audit(ctx, AuditEvent{
		Event:    EvRegister,
		ActorID:  id.ID,
		TargetID: id.ID,
		Email:    id.Email,
	})
	s.audit(ctx, AuditEvent{
		Event:    EvRoleDefaultGranted,
		ActorID:  id.ID,
		TargetID: id.ID,
		Detail:   map[string]any{"role": DefaultRole, "best_effort": true},
	})
	return id, nil
}
