package logic

import "context"

// DefaultRole is granted to every identity at creation time (password register
// and the OAuth implicit-register path). A missing grant only yields an empty
// roles claim, so the grant is best-effort.
const DefaultRole = "user"

// AdminRole is the elevated role gating the admin-only endpoints. Single-sourced
// here so the controller's admin guard and the catalog/migration 0006 stay in
// lockstep on the literal "admin" slug.
const AdminRole = "admin"

func (s *Service) GetRoles(ctx context.Context, identityID string) ([]string, error) {
	return s.store.GetRoles(ctx, identityID)
}

// GrantRole / RevokeRole are thin pass-throughs (catalog validation lives in the
// repo). A structured audit log would be emitted here; ⑥-audit will replace it
// with a persisted audit_logs row.
func (s *Service) GrantRole(ctx context.Context, identityID, slug string) error {
	if err := s.store.GrantRole(ctx, identityID, slug); err != nil {
		return err
	}
	// TODO(⑥-audit): persist role.granted audit event
	return nil
}

func (s *Service) RevokeRole(ctx context.Context, identityID, slug string) error {
	if err := s.store.RevokeRole(ctx, identityID, slug); err != nil {
		return err
	}
	// TODO(⑥-audit): persist role.revoked audit event
	return nil
}
