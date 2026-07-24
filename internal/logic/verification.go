package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"platform/services/identity/internal/iderr"
	identitypassword "platform/services/identity/internal/password"
	"platform/services/identity/internal/repo"
)

// newToken mints an opaque URL-safe token and its sha256-hex storage hash. Only
// the hash is persisted; the raw token lives solely in the emailed link.
func newToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RequestEmailVerification issues a verify-email link to the identity's email.
func (s *Service) RequestEmailVerification(ctx context.Context, identityID, ip string) error {
	acctKey, ipKey := "emailverify:id:"+identityID, "emailverify:ip:"+ip
	if locked, _ := s.store.Locked(ctx, acctKey); locked {
		return iderr.VerifyThrottled()
	}
	if locked, _ := s.store.Locked(ctx, ipKey); locked {
		return iderr.VerifyThrottled()
	}
	id, err := s.store.GetByID(ctx, identityID)
	if err != nil {
		return err
	}
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.CreateVerification(ctx, repo.NewVerificationInput{
		IdentityID: id.ID, Email: id.Email, Purpose: repo.PurposeVerifyEmail,
		TokenHash: hash, ExpiresAt: s.now().Add(s.cfg.VerifyTokenTTL),
	}); err != nil {
		return err
	}
	_ = s.store.RecordFailure(ctx, acctKey, s.cfg.VerifyResetWindow, s.cfg.VerifyResetLockFor, s.cfg.VerifyMaxReq)
	_ = s.store.RecordFailure(ctx, ipKey, s.cfg.VerifyResetWindow, s.cfg.VerifyResetLockFor, s.cfg.VerifyMaxReq)
	s.audit(ctx, AuditEvent{
		Event:    EvEmailVerifyRequested,
		ActorID:  identityID,
		TargetID: identityID,
		Email:    id.Email,
	})
	link := s.cfg.AccountBaseURL + "/verify-email?token=" + token
	if s.mailer != nil {
		return s.mailer.SendVerifyEmail(ctx, id.Email, link)
	}
	return nil
}

// VerifyEmail consumes a verify-email token and flips email_verified.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return iderr.VerificationInvalid()
	}
	rec, err := s.store.ConsumeVerification(ctx, hashToken(token), repo.PurposeVerifyEmail)
	if errors.Is(err, repo.ErrVerificationInvalid) {
		return iderr.VerificationInvalid()
	}
	if err != nil {
		return err
	}
	if err := s.store.SetEmailVerified(ctx, rec.IdentityID, true); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvEmailVerified,
		ActorID:  rec.IdentityID,
		TargetID: rec.IdentityID,
		Email:    rec.Email,
	})
	return nil
}

// RequestPasswordReset emails a reset link IF the account exists; it always
// returns nil to avoid account enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, email, ip string) error {
	email = CanonicalizeEmail(email)
	acctKey, ipKey := "pwreset:acct:"+email, "pwreset:ip:"+ip
	if locked, _ := s.store.Locked(ctx, acctKey); locked {
		return iderr.ResetThrottled()
	}
	if locked, _ := s.store.Locked(ctx, ipKey); locked {
		return iderr.ResetThrottled()
	}
	_ = s.store.RecordFailure(ctx, acctKey, s.cfg.VerifyResetWindow, s.cfg.VerifyResetLockFor, s.cfg.ResetMaxReq)
	_ = s.store.RecordFailure(ctx, ipKey, s.cfg.VerifyResetWindow, s.cfg.VerifyResetLockFor, s.cfg.ResetMaxReq)
	id, err := s.store.GetByEmail(ctx, email)
	if errors.Is(err, repo.ErrIdentityMissing) {
		s.audit(ctx, AuditEvent{
			Event:  EvPwResetRequested,
			Email:  email,
			Detail: map[string]any{"identity_found": false},
		})
		return nil // no leak
	}
	if err != nil {
		return err
	}
	token, hash, err := newToken()
	if err != nil {
		return err
	}
	if err := s.store.CreateVerification(ctx, repo.NewVerificationInput{
		IdentityID: id.ID, Email: id.Email, Purpose: repo.PurposePasswordReset,
		TokenHash: hash, ExpiresAt: s.now().Add(s.cfg.ResetTokenTTL),
	}); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{
		Event:    EvPwResetRequested,
		ActorID:  id.ID,
		TargetID: id.ID,
		Email:    id.Email,
		Detail:   map[string]any{"identity_found": true},
	})
	link := s.cfg.AccountBaseURL + "/reset?token=" + token
	if s.mailer != nil {
		return s.mailer.SendPasswordReset(ctx, id.Email, link)
	}
	return nil
}

// ResetPassword consumes a reset token, sets the new password, and force-logs-out
// all of the identity's sessions.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return iderr.VerificationInvalid()
	}
	// Reject length and global blocklist failures before consuming the
	// single-use token. Account-specific checks are repeated after resolving
	// the token's identity.
	normalized, err := s.preparePassword(
		ctx, newPassword, identitypassword.Context{},
	)
	if err != nil {
		return err
	}
	rec, err := s.store.ConsumeVerification(ctx, hashToken(token), repo.PurposePasswordReset)
	if errors.Is(err, repo.ErrVerificationInvalid) {
		return iderr.VerificationInvalid()
	}
	if err != nil {
		return err
	}
	normalized, err = s.preparePassword(ctx, normalized, identitypassword.Context{
		Email: rec.Email,
	})
	if err != nil {
		return err
	}
	hash, err := s.hashPassword(normalized)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePasswordHash(ctx, rec.IdentityID, hash); err != nil {
		return err
	}
	// Force-logout all sessions before auditing success: if the purge fails the
	// caller must see the error, and we must not claim a clean reset (matches the
	// "audit only after the final store op succeeds" pattern in VerifyEmail/Logout).
	if err := s.store.DeleteSessionsByIdentity(ctx, rec.IdentityID); err != nil {
		return err
	}
	if s.revoker != nil {
		if err := s.revoker.RevokeRefreshByIdentity(ctx, rec.IdentityID); err != nil {
			return err
		}
	}
	s.audit(ctx, AuditEvent{
		Event:    EvPwReset,
		ActorID:  rec.IdentityID,
		TargetID: rec.IdentityID,
		Email:    rec.Email,
	})
	return nil
}
