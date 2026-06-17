// Package cache implements the Redis-backed SessionStore and LoginThrottle
// (repo interfaces). Sessions are Redis-only (spec): a string per session
// plus a per-identity set index for list/revoke.
package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gredis"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// Redis wraps *gredis.Redis and satisfies repo.SessionStore and repo.LoginThrottle.
type Redis struct {
	c *gredis.Redis
}

// NewRedis creates a new Redis cache client.
func NewRedis(c *gredis.Redis) *Redis { return &Redis{c: c} }

func sessKey(id string) string        { return "sess:" + id }
func idxKey(identityID string) string { return "sess:idx:" + identityID }

// CreateSession stores s as a JSON string and adds its ID to the per-identity
// index set. If ttl > 0 the session key is set to expire after that duration.
func (r *Redis) CreateSession(ctx context.Context, s model.Session, ttl time.Duration) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err := r.c.Set(ctx, sessKey(s.ID), string(b)); err != nil {
		return err
	}
	if ttl > 0 {
		if _, err := r.c.Expire(ctx, sessKey(s.ID), int64(ttl.Seconds())); err != nil {
			return err
		}
	}
	_, err = r.c.SAdd(ctx, idxKey(s.IdentityID), s.ID)
	return err
}

// GetSession retrieves and unmarshals the session with the given id.
// Returns repo.ErrSessionNotFound when the key is absent or expired.
func (r *Redis) GetSession(ctx context.Context, id string) (model.Session, error) {
	v, err := r.c.Get(ctx, sessKey(id))
	if err != nil {
		return model.Session{}, err
	}
	if v.IsNil() {
		return model.Session{}, repo.ErrSessionNotFound
	}
	var s model.Session
	if err := json.Unmarshal(v.Bytes(), &s); err != nil {
		return model.Session{}, err
	}
	return s, nil
}

// DeleteSession removes the session key and prunes the per-identity index.
func (r *Redis) DeleteSession(ctx context.Context, id string) error {
	// Best-effort: remove from the identity index before deleting the session
	// key so we can look up the identityID while it still exists.
	s, err := r.GetSession(ctx, id)
	if err == nil {
		_, _ = r.c.SRem(ctx, idxKey(s.IdentityID), s.ID)
	}
	_, err = r.c.Del(ctx, sessKey(id))
	return err
}

// ListSessionsByIdentity returns all live sessions for identityID.
// Expired sessions found in the index are pruned lazily.
func (r *Redis) ListSessionsByIdentity(ctx context.Context, identityID string) ([]model.Session, error) {
	ids, err := r.c.SMembers(ctx, idxKey(identityID))
	if err != nil {
		return nil, err
	}
	var out []model.Session
	for _, idv := range ids.Strings() {
		s, err := r.GetSession(ctx, idv)
		if err == repo.ErrSessionNotFound {
			_, _ = r.c.SRem(ctx, idxKey(identityID), idv) // prune expired
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// DeleteSessionsByIdentity removes all sessions belonging to identityID and
// deletes the index key.
func (r *Redis) DeleteSessionsByIdentity(ctx context.Context, identityID string) error {
	ids, err := r.c.SMembers(ctx, idxKey(identityID))
	if err != nil {
		return err
	}
	for _, idv := range ids.Strings() {
		_, _ = r.c.Del(ctx, sessKey(idv))
	}
	_, err = r.c.Del(ctx, idxKey(identityID))
	return err
}

// Locked reports whether the given key is currently locked out.
func (r *Redis) Locked(ctx context.Context, key string) (bool, error) {
	n, err := r.c.Exists(ctx, "lock:"+key)
	return n > 0, err
}

// RecordFailure increments the failure counter for key. On the first increment
// a sliding window TTL is applied. Once failures reach max, a lock key is set
// that expires after lockDur.
func (r *Redis) RecordFailure(ctx context.Context, key string, window, lockDur time.Duration, max int) error {
	n, err := r.c.Incr(ctx, "fail:"+key)
	if err != nil {
		return err
	}
	if n == 1 {
		_, _ = r.c.Expire(ctx, "fail:"+key, int64(window.Seconds()))
	}
	if int(n) >= max {
		if _, err := r.c.Set(ctx, "lock:"+key, "1"); err != nil {
			return err
		}
		_, err = r.c.Expire(ctx, "lock:"+key, int64(lockDur.Seconds()))
		return err
	}
	return nil
}

// Reset clears the failure counter and any lock for the given key.
func (r *Redis) Reset(ctx context.Context, key string) error {
	_, err := r.c.Del(ctx, "fail:"+key, "lock:"+key)
	return err
}

// Compile-time interface assertions.
var _ repo.SessionStore = (*Redis)(nil)
var _ repo.LoginThrottle = (*Redis)(nil)
