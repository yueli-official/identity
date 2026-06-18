package logic

import (
	"context"
	"errors"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
	Locale      string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (model.Identity, error) {
	email := CanonicalizeEmail(in.Email)
	if err := ValidateEmail(email); err != nil {
		return model.Identity{}, iderr.InvalidEmail(email)
	}
	if err := ValidatePasswordStrength(in.Password); err != nil {
		return model.Identity{}, iderr.WeakPassword(err.Error())
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return model.Identity{}, err
	}
	id, err := s.store.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{
		Email: email, DisplayName: in.DisplayName, Locale: in.Locale, PasswordHash: hash,
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
