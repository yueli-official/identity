package logic

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	"platform/services/identity/internal/iderr"
)

const bcryptCost = 12
const minPasswordLen = 8

// dummyBcryptHash equalizes login timing on the unknown-email path to mitigate
// account enumeration. Computed once at package init.
var dummyBcryptHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalization-placeholder"), bcryptCost)

// VerifyDummy runs a bcrypt comparison against a constant hash purely to spend
// the same time as a real verification (timing equalization). Result is ignored.
func VerifyDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(plain))
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func ValidatePasswordStrength(plain string) error {
	if len(plain) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	return nil
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
	if !VerifyPassword(hash, current) {
		return iderr.InvalidCredentials()
	}
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return iderr.WeakPassword(err.Error())
	}
	newHash, err := HashPassword(newPassword)
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
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return iderr.WeakPassword(err.Error())
	}
	hash, err := HashPassword(newPassword)
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
