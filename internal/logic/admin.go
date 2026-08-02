package logic

import (
	"context"
	"errors"

	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/model"
	identitypassword "github.com/yueli-official/identity/internal/password"
	"github.com/yueli-official/identity/internal/repo"
)

// AdminListUsers serves the admin user-management list (filtered + paginated).
func (s *Service) AdminListUsers(ctx context.Context, f repo.AdminUserFilter) ([]repo.AdminUserRow, int, error) {
	return s.store.AdminListUsers(ctx, f)
}

// AdminUserStats returns identity counts per status for the admin dashboard.
func (s *Service) AdminUserStats(ctx context.Context) (map[string]int, error) {
	return s.store.AdminUserStatusCounts(ctx)
}

// AdminGetUser returns a single user's admin view (identity + profile + roles).
func (s *Service) AdminGetUser(ctx context.Context, targetID string) (repo.AdminUserRow, error) {
	idn, err := s.store.GetByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repo.ErrIdentityMissing) {
			return repo.AdminUserRow{}, iderr.IdentityNotFound()
		}
		return repo.AdminUserRow{}, err
	}
	prof, _ := s.store.GetProfile(ctx, targetID) // empty profile acceptable
	roles, err := s.store.GetRoles(ctx, targetID)
	if err != nil {
		return repo.AdminUserRow{}, err
	}
	return repo.AdminUserRow{
		InternalID:     idn.ID,
		UserKey:        idn.UserKey,
		Email:          idn.Email,
		EmailVerified:  idn.EmailVerified,
		Status:         idn.Status,
		CreatedAt:      idn.CreatedAt,
		DisplayName:    prof.DisplayName,
		Handle:         prof.Handle,
		AvatarMediaKey: prof.AvatarMediaKey,
		Roles:          roles,
	}, nil
}

// AdminSetUserStatus changes a target's lifecycle status. Banning (disabled) or
// soft-deleting also revokes the target's sessions and OIDC refresh tokens so
// the change takes effect immediately rather than only at next login. An admin
// cannot change their own status (self-lockout guard). The acting admin id is
// read from ctx (injected by the controller's admin guard).
func (s *Service) AdminSetUserStatus(ctx context.Context, targetID string, status model.Status) error {
	if status != model.StatusActive && status != model.StatusDisabled && status != model.StatusDeleted {
		return iderr.InvalidStatus(string(status))
	}
	adminID := actor.From(ctx).IdentityID
	if targetID == adminID {
		return iderr.SelfAdminTarget()
	}
	current, err := s.store.GetByID(ctx, targetID)
	if errors.Is(err, repo.ErrIdentityMissing) {
		return iderr.IdentityNotFound()
	}
	if err != nil {
		return err
	}
	// Deleted is terminal: its email may already have been claimed by a new
	// identity under the partial unique index. Reanimation would either fail at
	// storage time or create an ambiguous account lifecycle.
	if current.Status == model.StatusDeleted && status != model.StatusDeleted {
		return iderr.InvalidStatus(string(status))
	}
	if err := s.store.SetIdentityStatus(ctx, targetID, status); err != nil {
		if errors.Is(err, repo.ErrIdentityMissing) {
			return iderr.IdentityNotFound()
		}
		return err
	}
	if status != model.StatusActive {
		s.killSessions(ctx, targetID)
	}
	event := EvAdminStatusChanged
	if status == model.StatusDeleted {
		event = EvAdminUserDeleted
	}
	s.audit(ctx, AuditEvent{
		Event:    event,
		ActorID:  adminID,
		TargetID: targetID,
		Detail:   map[string]any{"status": string(status)},
	})
	return nil
}

// AdminDeleteUser soft-deletes an identity (status='deleted', which frees the
// email and bars login). Thin wrapper over AdminSetUserStatus.
func (s *Service) AdminDeleteUser(ctx context.Context, targetID string) error {
	return s.AdminSetUserStatus(ctx, targetID, model.StatusDeleted)
}

// AdminResetPassword sets a new password for the target (no current-password
// check — this is the admin override path) and revokes the target's sessions so
// the old credential cannot keep a live session. Validates strength.
func (s *Service) AdminResetPassword(ctx context.Context, targetID, newPassword string) error {
	identity, err := s.store.GetByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, repo.ErrIdentityMissing) {
			return iderr.IdentityNotFound()
		}
		return err
	}
	normalized, err := s.preparePassword(ctx, newPassword, identitypassword.Context{
		Email: identity.Email,
	})
	if err != nil {
		return err
	}
	hash, err := s.hashPassword(normalized)
	if err != nil {
		return err
	}
	if err := s.store.SetPasswordHash(ctx, targetID, hash); err != nil {
		return err
	}
	s.killSessions(ctx, targetID)
	s.audit(ctx, AuditEvent{
		Event:    EvAdminPasswordReset,
		ActorID:  actor.From(ctx).IdentityID,
		TargetID: targetID,
	})
	return nil
}

// AdminCreateUser provisions a new account from the admin console: validates
// email + password, creates the identity, and grants the requested roles
// (defaulting to {user}). Returns the created identity.
func (s *Service) AdminCreateUser(ctx context.Context, in RegisterInput, roles []string) (model.Identity, error) {
	requested := make([]string, 0, len(roles)+1)
	seen := map[string]bool{}
	for _, role := range append([]string{DefaultRole}, roles...) {
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		requested = append(requested, role)
	}
	in.Roles = requested
	id, err := s.Register(ctx, in)
	if err != nil {
		return model.Identity{}, err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvAdminUserCreated,
		ActorID:  actor.From(ctx).IdentityID,
		TargetID: id.ID,
		Email:    id.Email,
		Detail:   map[string]any{"roles": roles},
	})
	return id, nil
}

// killSessions revokes all of an identity's IdP sessions and OIDC refresh tokens
// (best-effort; failures are swallowed so the primary mutation still succeeds).
func (s *Service) killSessions(ctx context.Context, identityID string) {
	_ = s.store.DeleteSessionsByIdentity(ctx, identityID)
	_ = s.revokeRefreshByIdentity(ctx, identityID)
}
