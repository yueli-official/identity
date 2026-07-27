package logic

import (
	"context"

	"github.com/yueli-official/identity/internal/iderr"
	identitypassword "github.com/yueli-official/identity/internal/password"
)

var defaultPasswordManager = identitypassword.New(identitypassword.DefaultConfig())

// dummyPasswordHash equalizes login timing on the unknown-email path to mitigate
// account enumeration. Computed once at package init.
var dummyPasswordHash, _ = defaultPasswordManager.Hash("timing-equalization-placeholder")

// VerifyDummy runs a comparison against a constant hash purely to spend
// the same time as a real verification (timing equalization). Result is ignored.
func VerifyDummy(plain string) {
	_ = defaultPasswordManager.Verify(dummyPasswordHash, plain)
}

func HashPassword(plain string) (string, error) {
	return defaultPasswordManager.Hash(identitypassword.Normalize(plain))
}

func VerifyPassword(hash, plain string) bool {
	return defaultPasswordManager.Verify(hash, plain)
}

func ValidatePasswordStrength(plain string) error {
	_, err := defaultPasswordManager.Validate(
		context.Background(), plain, identitypassword.Context{},
	)
	return err
}

func (s *Service) preparePassword(
	ctx context.Context,
	plain string,
	account identitypassword.Context,
) (string, error) {
	normalized, err := s.passwords.Validate(ctx, plain, account)
	if err != nil {
		reason := identitypassword.ParseReason(err)
		if reason != "" {
			return "", iderr.WeakPassword(reason)
		}
		return "", err
	}
	return normalized, nil
}

func (s *Service) hashPassword(normalized string) (string, error) {
	return s.passwords.Hash(normalized)
}

func (s *Service) PasswordPolicy() identitypassword.Policy {
	return s.passwords.Policy()
}

// ChangePassword verifies the caller's current password, sets a new one, and
// revokes all OTHER sessions (keeping currentSessionID so the caller stays
// logged in here). Mirrors the reset flow's "force logout other sessions"
// but for the authenticated change-password path.
func (s *Service) ChangePassword(ctx context.Context, identityID, currentSessionID, current, newPassword string) error {
	hash, err := s.store.GetPasswordHash(ctx, identityID)
	if err != nil {
		return err
	}
	if !s.passwords.Verify(hash, current) {
		return iderr.InvalidCredentials()
	}
	identity, err := s.store.GetByID(ctx, identityID)
	if err != nil {
		return err
	}
	normalized, err := s.preparePassword(ctx, newPassword, identitypassword.Context{
		Email: identity.Email,
	})
	if err != nil {
		return err
	}
	newHash, err := s.hashPassword(normalized)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePasswordHash(ctx, identityID, newHash); err != nil {
		return err
	}
	s.revokeOtherSessions(ctx, identityID, currentSessionID)
	s.audit(ctx, AuditEvent{Event: EvPasswordChanged, ActorID: identityID, TargetID: identityID})
	return nil
}

// SetPassword sets an INITIAL password for an identity that has none (e.g. an
// OAuth-only account adding a password so it can later unbind its OAuth login).
// It does NOT require the current password; an account that already has one must
// use ChangePassword instead (which verifies the current password). No forced
// logout: there was no prior password whose compromise we'd be containing.
func (s *Service) SetPassword(ctx context.Context, identityID, newPassword string) error {
	has, err := s.hasPassword(ctx, identityID)
	if err != nil {
		return err
	}
	if has {
		return iderr.PasswordAlreadySet()
	}
	identity, err := s.store.GetByID(ctx, identityID)
	if err != nil {
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
	if err := s.store.SetPasswordHash(ctx, identityID, hash); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{Event: EvPasswordSet, ActorID: identityID, TargetID: identityID})
	return nil
}

// revokeOtherSessions clears every session of identityID except keepID and
// revokes their bound refresh tokens. Best-effort: a per-session failure does
// not abort the rest (the password is already changed).
func (s *Service) revokeOtherSessions(ctx context.Context, identityID, keepID string) {
	sessions, err := s.store.ListSessionsByIdentity(ctx, identityID)
	if err != nil {
		return
	}
	for _, sess := range sessions {
		if sess.ID == keepID {
			continue
		}
		if s.revoker != nil {
			_ = s.revoker.RevokeRefreshBySession(ctx, sess.ID)
		}
		_ = s.store.DeleteSession(ctx, sess.ID)
	}
}
