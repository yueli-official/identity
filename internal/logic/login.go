package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

type LoginOutput struct {
	SessionID string
	Identity  model.Identity
}

func (s *Service) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	email := CanonicalizeEmail(in.Email)
	acctKey := "login:acct:" + email
	ipKey := "login:ip:" + in.IP

	// Lockout check first (account + IP), before any password work.
	if locked, _ := s.store.Locked(ctx, acctKey); locked {
		return LoginOutput{}, iderr.AccountLocked()
	}
	if locked, _ := s.store.Locked(ctx, ipKey); locked {
		return LoginOutput{}, iderr.AccountLocked()
	}

	fail := func() (LoginOutput, error) {
		_ = s.store.RecordFailure(ctx, acctKey, s.cfg.LoginFailWindow, s.cfg.LoginLockFor, s.cfg.LoginMaxFails)
		_ = s.store.RecordFailure(ctx, ipKey, s.cfg.LoginFailWindow, s.cfg.LoginLockFor, s.cfg.IPMaxFails)
		return LoginOutput{}, iderr.InvalidCredentials()
	}

	id, err := s.store.GetByEmail(ctx, email)
	if errors.Is(err, repo.ErrIdentityMissing) {
		VerifyDummy(in.Password) // equalize timing vs the wrong-password path
		return fail()
	}
	if err != nil {
		return LoginOutput{}, err
	}
	hash, err := s.store.GetPasswordHash(ctx, id.ID)
	if err != nil || !VerifyPassword(hash, in.Password) {
		return fail()
	}
	if id.Status == model.StatusDisabled {
		return LoginOutput{}, iderr.AccountDisabled()
	}
	if id.Status == model.StatusDeleted {
		return fail()
	}

	// Success: clear counters, mint a fresh (rotated) session id.
	_ = s.store.Reset(ctx, acctKey)
	_ = s.store.Reset(ctx, ipKey)
	sess := model.Session{
		ID: uuid.NewString(), IdentityID: id.ID,
		CreatedAt: s.now(), LastSeen: s.now(), UserAgent: in.UserAgent, IP: in.IP,
	}
	if err := s.store.CreateSession(ctx, sess, s.cfg.SessionIdleTTL); err != nil {
		return LoginOutput{}, err
	}
	return LoginOutput{SessionID: sess.ID, Identity: id}, nil
}
