package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

type OAuthLoginInput struct {
	Provider      string
	ProviderUID   string
	Email         string
	EmailVerified bool
	DisplayName   string
	Locale        string
	UserAgent     string
	IP            string
}

// OAuthLogin resolves (or links/creates) the identity behind an external provider
// then mints a session identical to password login.
func (s *Service) OAuthLogin(ctx context.Context, in OAuthLoginInput) (LoginOutput, error) {
	email := CanonicalizeEmail(in.Email)

	var created bool
	id, err := s.store.GetByProviderUID(ctx, in.Provider, in.ProviderUID)
	switch {
	case err == nil:
		// already linked → straight in
	case errors.Is(err, repo.ErrIdentityMissing):
		id, created, err = s.resolveOAuthIdentity(ctx, in, email)
		if err != nil {
			return LoginOutput{}, err
		}
	default:
		return LoginOutput{}, err
	}

	if id.Status == model.StatusDisabled {
		return LoginOutput{}, iderr.AccountDisabled()
	}
	if id.Status == model.StatusDeleted {
		return LoginOutput{}, iderr.AccountDisabled()
	}

	authenticatedAt := s.now().UTC()
	primary := authentication.Federated(
		uuid.NewString(), authenticatedAt, in.Provider+":"+in.ProviderUID,
	)
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
		Event:    EvOAuthLogin,
		ActorID:  id.ID,
		TargetID: id.ID,
		Email:    id.Email,
		Detail:   map[string]any{"provider": in.Provider, "created": created},
	})
	return LoginOutput{SessionID: sess.ID, Identity: id}, nil
}

// resolveOAuthIdentity resolves an identity for an OAuth login where no existing
// credential link was found. Returns the resolved identity, a bool indicating
// whether it was newly created (implicit-register), and any error.
func (s *Service) resolveOAuthIdentity(ctx context.Context, in OAuthLoginInput, email string) (model.Identity, bool, error) {
	if email == "" {
		return model.Identity{}, false, iderr.OAuthNoEmail()
	}
	existing, gerr := s.store.GetByEmail(ctx, email)
	switch {
	case gerr == nil:
		// verified email matches an existing identity → link; unverified → refuse.
		if !in.EmailVerified {
			return model.Identity{}, false, iderr.OAuthEmailConflict(email)
		}
		if existing.Status != model.StatusActive {
			return model.Identity{}, false, iderr.AccountDisabled()
		}
		if lerr := s.store.LinkOAuthCredential(ctx, existing.ID, in.Provider, in.ProviderUID, email, true); lerr != nil {
			return model.Identity{}, false, lerr
		}
		s.audit(ctx, AuditEvent{
			Event:    EvCredentialLinked,
			ActorID:  existing.ID,
			TargetID: existing.ID,
			Email:    existing.Email,
			Detail:   map[string]any{"provider": in.Provider},
		})
		return existing, false, nil
	case errors.Is(gerr, repo.ErrIdentityMissing):
		// no collision → implicit register (NEW identity, not a link)
		id, cerr := s.store.CreateOAuthIdentity(ctx, repo.NewOAuthIdentityInput{
			Email: email, EmailVerified: in.EmailVerified, DisplayName: in.DisplayName,
			Locale: in.Locale, Provider: in.Provider, ProviderUID: in.ProviderUID,
		})
		if cerr != nil {
			return model.Identity{}, false, cerr
		}
		_ = s.store.GrantRole(ctx, id.ID, DefaultRole) // best-effort default role (create path only)
		s.audit(ctx, AuditEvent{
			Event:    EvRoleDefaultGranted,
			ActorID:  id.ID,
			TargetID: id.ID,
			Detail:   map[string]any{"role": DefaultRole, "best_effort": true},
		})
		return id, true, nil
	default:
		return model.Identity{}, false, gerr
	}
}
