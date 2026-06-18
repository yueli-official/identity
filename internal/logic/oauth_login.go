package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
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
// per spec §10, then mints a session identical to password login.
func (s *Service) OAuthLogin(ctx context.Context, in OAuthLoginInput) (LoginOutput, error) {
	email := CanonicalizeEmail(in.Email)

	id, err := s.store.GetByProviderUID(ctx, in.Provider, in.ProviderUID)
	switch {
	case err == nil:
		// already linked → straight in
	case errors.Is(err, repo.ErrIdentityMissing):
		id, err = s.resolveOAuthIdentity(ctx, in, email)
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

	sess := model.Session{
		ID: uuid.NewString(), IdentityID: id.ID,
		CreatedAt: s.now(), LastSeen: s.now(), UserAgent: in.UserAgent, IP: in.IP,
	}
	if err := s.store.CreateSession(ctx, sess, s.cfg.SessionIdleTTL); err != nil {
		return LoginOutput{}, err
	}
	return LoginOutput{SessionID: sess.ID, Identity: id}, nil
}

func (s *Service) resolveOAuthIdentity(ctx context.Context, in OAuthLoginInput, email string) (model.Identity, error) {
	if email == "" {
		return model.Identity{}, iderr.OAuthNoEmail()
	}
	existing, gerr := s.store.GetByEmail(ctx, email)
	switch {
	case gerr == nil:
		// §10: verified email matches an existing identity → link; unverified → refuse.
		if !in.EmailVerified {
			return model.Identity{}, iderr.OAuthEmailConflict(email)
		}
		if lerr := s.store.LinkOAuthCredential(ctx, existing.ID, in.Provider, in.ProviderUID, email, true); lerr != nil {
			return model.Identity{}, lerr
		}
		return existing, nil
	case errors.Is(gerr, repo.ErrIdentityMissing):
		// no collision → implicit register (NEW identity, not a link)
		id, cerr := s.store.CreateOAuthIdentity(ctx, repo.NewOAuthIdentityInput{
			Email: email, EmailVerified: in.EmailVerified, DisplayName: in.DisplayName,
			Locale: in.Locale, Provider: in.Provider, ProviderUID: in.ProviderUID,
		})
		if cerr != nil {
			return model.Identity{}, cerr
		}
		_ = s.store.GrantRole(ctx, id.ID, DefaultRole) // best-effort default role (create path only)
		return id, nil
	default:
		return model.Identity{}, gerr
	}
}
