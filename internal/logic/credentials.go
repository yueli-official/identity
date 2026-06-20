package logic

import (
	"context"
	"errors"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/repo"
)

// CredentialSummary describes the login credentials bound to an identity.
type CredentialSummary struct {
	HasPassword bool
	OAuth       []repo.OAuthCredential
}

// ListCredentials returns the identity's password + oauth credentials.
func (s *Service) ListCredentials(ctx context.Context, identityID string) (CredentialSummary, error) {
	oauth, err := s.store.ListOAuthCredentials(ctx, identityID)
	if err != nil {
		return CredentialSummary{}, err
	}
	hasPw, err := s.hasPassword(ctx, identityID)
	if err != nil {
		return CredentialSummary{}, err
	}
	return CredentialSummary{HasPassword: hasPw, OAuth: oauth}, nil
}

// hasPassword reports whether the identity has a (non-empty) password credential.
func (s *Service) hasPassword(ctx context.Context, identityID string) (bool, error) {
	hash, err := s.store.GetPasswordHash(ctx, identityID)
	if errors.Is(err, repo.ErrIdentityMissing) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return hash != "", nil
}

// BindOAuth links an external-provider account to identityID. Idempotent if the
// same identity already owns the link; rejects when the (provider, uid) is bound
// to a DIFFERENT identity (no silent account takeover).
func (s *Service) BindOAuth(ctx context.Context, identityID, provider, providerUID, email string, emailVerified bool) error {
	switch existing, err := s.store.GetByProviderUID(ctx, provider, providerUID); {
	case err == nil:
		if existing.ID == identityID {
			return nil // already mine
		}
		return iderr.CredentialConflict(provider)
	case errors.Is(err, repo.ErrIdentityMissing):
		// not linked yet → proceed
	default:
		return err
	}
	if err := s.store.LinkOAuthCredential(ctx, identityID, provider, providerUID, email, emailVerified); err != nil {
		if errors.Is(err, repo.ErrProviderUIDTaken) {
			return iderr.CredentialConflict(provider) // lost a race
		}
		return err
	}
	s.audit(ctx, AuditEvent{
		Event: EvCredentialLinked, ActorID: identityID, TargetID: identityID,
		Detail: map[string]any{"provider": provider},
	})
	return nil
}

// UnbindCredential removes an oauth provider credential, refusing to remove the
// LAST login credential ("不得解绑最后一个可登录凭据").
func (s *Service) UnbindCredential(ctx context.Context, identityID, provider string) error {
	summary, err := s.ListCredentials(ctx, identityID)
	if err != nil {
		return err
	}
	total := len(summary.OAuth)
	if summary.HasPassword {
		total++
	}
	if total <= 1 {
		return iderr.LastCredential()
	}
	ok, err := s.store.DeleteOAuthCredential(ctx, identityID, provider)
	if err != nil {
		return err
	}
	if !ok {
		return iderr.CredentialNotFound()
	}
	s.audit(ctx, AuditEvent{
		Event: EvCredentialUnlinked, ActorID: identityID, TargetID: identityID,
		Detail: map[string]any{"provider": provider},
	})
	return nil
}
