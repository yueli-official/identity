package logic

import (
	"context"
	"errors"

	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// AuthenticatedSession resolves the server-side session and its active identity.
// Callers that need assurance facts must use the returned Session rather than
// reconstructing authentication time from request/token timestamps.
func (s *Service) AuthenticatedSession(ctx context.Context, sessionID string) (model.Session, model.Identity, error) {
	if sessionID == "" {
		return model.Session{}, model.Identity{}, iderr.NotAuthenticated()
	}
	sess, err := s.store.GetSession(ctx, sessionID)
	if errors.Is(err, repo.ErrSessionNotFound) {
		return model.Session{}, model.Identity{}, iderr.NotAuthenticated()
	}
	if err != nil {
		return model.Session{}, model.Identity{}, err
	}
	id, err := s.store.GetByID(ctx, sess.IdentityID)
	if err != nil {
		return model.Session{}, model.Identity{}, iderr.NotAuthenticated()
	}
	if id.Status != model.StatusActive {
		return model.Session{}, model.Identity{}, iderr.NotAuthenticated()
	}
	return sess, id, nil
}

// Me resolves a session id to its identity (the account-center /session/me).
func (s *Service) Me(ctx context.Context, sessionID string) (model.Identity, error) {
	_, identity, err := s.AuthenticatedSession(ctx, sessionID)
	return identity, err
}

// RequireAuthentication evaluates a server-side session context against a
// typed requirement. It never trusts request timestamps or token issuance time.
func (s *Service) RequireAuthentication(
	ctx context.Context,
	sessionID string,
	requirement authentication.Requirement,
) (model.Session, model.Identity, error) {
	session, identity, err := s.AuthenticatedSession(ctx, sessionID)
	if err != nil {
		return model.Session{}, model.Identity{}, err
	}
	decision := authentication.Evaluate(session.Authentication, requirement, s.now())
	if !decision.Satisfied {
		return model.Session{}, model.Identity{}, iderr.StepUpRequired(decision.Missing)
	}
	return session, identity, nil
}

// GetByID fetches a single identity by its primary-key ID.
func (s *Service) GetByID(ctx context.Context, id string) (model.Identity, error) {
	return s.store.GetByID(ctx, id)
}

// GetByEmail fetches a single identity by email (canonicalized first, matching
// the login/register path). Returns repo.ErrIdentityMissing if none exists. Used
// by the startup bootstrap-admin grant.
func (s *Service) GetByEmail(ctx context.Context, email string) (model.Identity, error) {
	return s.store.GetByEmail(ctx, CanonicalizeEmail(email))
}

// GetProfile fetches the profile for an identity by its primary-key ID.
func (s *Service) GetProfile(ctx context.Context, id string) (model.Profile, error) {
	return s.store.GetProfile(ctx, id)
}

// Logout clears a single session and revokes the refresh tokens bound to it.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	// Resolve the identity before deleting so we can audit with the correct actor/target.
	// If the lookup fails we still proceed (don't block logout) and record "" identity.
	var identityID string
	if sess, err := s.store.GetSession(ctx, sessionID); err == nil {
		identityID = sess.IdentityID
	}

	if s.revoker != nil {
		if err := s.revoker.RevokeRefreshBySession(ctx, sessionID); err != nil {
			return err
		}
	}
	if err := s.store.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvLogout,
		ActorID:  identityID,
		TargetID: identityID,
		Detail:   map[string]any{"session_id": sessionID},
	})
	return nil
}

// ListSessions lists an identity's active IdP sessions (account-center).
func (s *Service) ListSessions(ctx context.Context, identityID string) ([]model.Session, error) {
	return s.store.ListSessionsByIdentity(ctx, identityID)
}

// RevokeSession clears one session and revokes its bound refresh tokens, but
// only if it belongs to identityID. A not-found session and a session owned by
// someone else are merged into the same error so a caller cannot probe another
// account's session ids.
func (s *Service) RevokeSession(ctx context.Context, identityID, sessionID string) error {
	sess, err := s.store.GetSession(ctx, sessionID)
	if errors.Is(err, repo.ErrSessionNotFound) || (err == nil && sess.IdentityID != identityID) {
		return iderr.SessionNotFound()
	}
	if err != nil {
		return err
	}
	if s.revoker != nil {
		if err := s.revoker.RevokeRefreshBySession(ctx, sessionID); err != nil {
			return err
		}
	}
	if err := s.store.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvSessionRevoked,
		ActorID:  identityID,
		TargetID: identityID,
		Detail:   map[string]any{"session_id": sessionID},
	})
	return nil
}

// LogoutAll clears all of an identity's sessions and revokes all its refresh tokens.
func (s *Service) LogoutAll(ctx context.Context, identityID string) error {
	if s.revoker != nil {
		if err := s.revoker.RevokeRefreshByIdentity(ctx, identityID); err != nil {
			return err
		}
	}
	if err := s.store.DeleteSessionsByIdentity(ctx, identityID); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvLogoutAll,
		ActorID:  identityID,
		TargetID: identityID,
		Detail:   map[string]any{"scope": "all"},
	})
	return nil
}
