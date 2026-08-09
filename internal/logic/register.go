package logic

import (
	"context"
	"errors"

	"github.com/yueli-official/foundation/go/abuse"
	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/identity/internal/identityabuse"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/model"
	identitypassword "github.com/yueli-official/identity/internal/password"
	"github.com/yueli-official/identity/internal/repo"
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
	Roles       []string // trusted provisioning path; empty defaults to {user}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (model.Identity, error) {
	email := CanonicalizeEmail(in.Email)
	if in.AttemptID != "" || in.IP != "" {
		attemptID := in.AttemptID
		if attemptID == "" {
			attemptID = identifier.MustNew().String()
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
			return model.Identity{}, iderr.AccountLockedUntil(admission.RetryAt)
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
	roles := in.Roles
	if len(roles) == 0 {
		roles = []string{DefaultRole}
	}
	id, err := s.store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		ID: in.ID, Email: email, DisplayName: in.DisplayName, Locale: in.Locale, PasswordHash: hash,
		Roles: roles,
	})
	if errors.Is(err, repo.ErrEmailTaken) {
		return model.Identity{}, iderr.EmailTaken(email)
	}
	if err != nil {
		var unknown repo.UnknownRoleError
		if errors.As(err, &unknown) {
			return model.Identity{}, iderr.UnknownRole(unknown.Slug)
		}
		return model.Identity{}, err
	}
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
		Detail:   map[string]any{"role": DefaultRole},
	})
	return id, nil
}
