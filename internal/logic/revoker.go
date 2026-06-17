package logic

import "context"

// RefreshRevoker revokes OIDC refresh tokens bound to a session/identity. The
// concrete impl lives in package oidc (Store); logic depends only on this seam
// so passive logout can revoke refresh tokens without importing oidc.
type RefreshRevoker interface {
	RevokeRefreshBySession(ctx context.Context, sessionID string) error
	RevokeRefreshByIdentity(ctx context.Context, identityID string) error
}
