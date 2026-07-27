package repo

import (
	"context"
	"errors"
	"time"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/model"
)

// NewRecoveringSessionStore keeps Redis as the hot path while using durable as
// the source of truth. If Redis loses a session key, reads recover from durable
// storage and repopulate Redis with the remaining TTL.
func NewRecoveringSessionStore(cache SessionStore, durable SessionStore) SessionStore {
	return &recoveringSessionStore{cache: cache, durable: durable}
}

type recoveringSessionStore struct {
	cache   SessionStore
	durable SessionStore
}

func (s *recoveringSessionStore) CreateSession(ctx context.Context, sess model.Session, ttl time.Duration) error {
	sess = withSessionExpiry(sess, ttl)
	if err := s.durable.CreateSession(ctx, sess, ttl); err != nil {
		return err
	}
	_ = s.cache.CreateSession(ctx, sess, ttl)
	return nil
}

func (s *recoveringSessionStore) GetSession(ctx context.Context, id string) (model.Session, error) {
	sess, err := s.cache.GetSession(ctx, id)
	if err == nil {
		return sess, nil
	}

	sess, derr := s.durable.GetSession(ctx, id)
	if derr != nil {
		if errors.Is(derr, ErrSessionNotFound) {
			return model.Session{}, derr
		}
		return model.Session{}, derr
	}
	if ttl := remainingSessionTTL(sess); ttl > 0 {
		_ = s.cache.CreateSession(ctx, sess, ttl)
	}
	return sess, nil
}

func (s *recoveringSessionStore) DeleteSession(ctx context.Context, id string) error {
	_ = s.cache.DeleteSession(ctx, id)
	return s.durable.DeleteSession(ctx, id)
}

func (s *recoveringSessionStore) ListSessionsByIdentity(ctx context.Context, identityID string) ([]model.Session, error) {
	return s.durable.ListSessionsByIdentity(ctx, identityID)
}

func (s *recoveringSessionStore) DeleteSessionsByIdentity(ctx context.Context, identityID string) error {
	_ = s.cache.DeleteSessionsByIdentity(ctx, identityID)
	return s.durable.DeleteSessionsByIdentity(ctx, identityID)
}

func withSessionExpiry(sess model.Session, ttl time.Duration) model.Session {
	sess.Authentication = authentication.NormalizeLegacy(sess.Authentication, sess.CreatedAt)
	if ttl <= 0 || !sess.ExpiresAt.IsZero() {
		return sess
	}
	base := sess.CreatedAt
	if base.IsZero() {
		base = time.Now().UTC()
	}
	sess.ExpiresAt = base.Add(ttl)
	return sess
}

func remainingSessionTTL(sess model.Session) time.Duration {
	if sess.ExpiresAt.IsZero() {
		return 0
	}
	return time.Until(sess.ExpiresAt)
}
