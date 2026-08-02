package logic

import "context"

// RefreshRevoker revokes OIDC refresh tokens bound to a session/identity. The
// concrete impl lives in package oidc (Store); logic depends only on this seam
// so passive logout can revoke refresh tokens without importing oidc.
type RefreshRevoker interface {
	RevokeRefreshBySession(ctx context.Context, sessionID string) error
	RevokeRefreshBySubject(ctx context.Context, subject string) error
}

func (s *Service) revokeRefreshByIdentity(ctx context.Context, identityID string) error {
	if s.revoker == nil {
		return nil
	}
	subjects, err := s.store.ListOIDCSubjects(ctx, identityID)
	if err != nil {
		return err
	}
	for _, subject := range subjects {
		if err := s.revoker.RevokeRefreshBySubject(ctx, subject); err != nil {
			return err
		}
	}
	return nil
}
