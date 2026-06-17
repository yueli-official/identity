package logic

import (
	"context"
	"errors"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// Me resolves a session id to its identity (the account-center /session/me).
func (s *Service) Me(ctx context.Context, sessionID string) (model.Identity, error) {
	if sessionID == "" {
		return model.Identity{}, iderr.NotAuthenticated()
	}
	sess, err := s.store.GetSession(ctx, sessionID)
	if errors.Is(err, repo.ErrSessionNotFound) {
		return model.Identity{}, iderr.NotAuthenticated()
	}
	if err != nil {
		return model.Identity{}, err
	}
	id, err := s.store.GetByID(ctx, sess.IdentityID)
	if err != nil {
		return model.Identity{}, iderr.NotAuthenticated()
	}
	return id, nil
}

// Logout clears a single session. (Session-bound refresh-token revocation is
// milestone 4; no refresh tokens exist yet.)
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.store.DeleteSession(ctx, sessionID)
}

// ListSessions lists an identity's active IdP sessions (account-center).
func (s *Service) ListSessions(ctx context.Context, identityID string) ([]model.Session, error) {
	return s.store.ListSessionsByIdentity(ctx, identityID)
}

// LogoutAll clears all of an identity's sessions ("log out everywhere").
func (s *Service) LogoutAll(ctx context.Context, identityID string) error {
	return s.store.DeleteSessionsByIdentity(ctx, identityID)
}
