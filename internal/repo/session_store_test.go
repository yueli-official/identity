package repo_test

import (
	"context"
	"testing"
	"time"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

func TestRecoveringSessionStoreRestoresCacheFromDurableStore(t *testing.T) {
	ctx := context.Background()
	cache := newMapSessionStore()
	durable := newMapSessionStore()
	sess := model.Session{
		ID:         "00000000-0000-0000-0000-000000000001",
		IdentityID: "00000000-0000-0000-0000-000000000002",
		CreatedAt:  time.Now().Add(-time.Minute),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	durable.sessions[sess.ID] = sess

	store := repo.NewRecoveringSessionStore(cache, durable)
	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IdentityID != sess.IdentityID {
		t.Fatalf("identity = %q, want %q", got.IdentityID, sess.IdentityID)
	}
	if _, err := cache.GetSession(ctx, sess.ID); err != nil {
		t.Fatalf("cache was not restored: %v", err)
	}
}

func TestRecoveringSessionStoreWritesDurableFirst(t *testing.T) {
	ctx := context.Background()
	cache := newMapSessionStore()
	durable := newMapSessionStore()
	store := repo.NewRecoveringSessionStore(cache, durable)
	sess := model.Session{
		ID:         "00000000-0000-0000-0000-000000000003",
		IdentityID: "00000000-0000-0000-0000-000000000004",
		CreatedAt:  time.Now(),
	}

	if err := store.CreateSession(ctx, sess, time.Hour); err != nil {
		t.Fatal(err)
	}
	got, err := durable.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpiresAt.IsZero() {
		t.Fatal("durable session missing expiry")
	}
	if _, err := cache.GetSession(ctx, sess.ID); err != nil {
		t.Fatalf("cache session missing: %v", err)
	}
}

type mapSessionStore struct {
	sessions map[string]model.Session
}

func newMapSessionStore() *mapSessionStore {
	return &mapSessionStore{sessions: map[string]model.Session{}}
}

func (s *mapSessionStore) CreateSession(_ context.Context, sess model.Session, _ time.Duration) error {
	s.sessions[sess.ID] = sess
	return nil
}

func (s *mapSessionStore) GetSession(_ context.Context, id string) (model.Session, error) {
	sess, ok := s.sessions[id]
	if !ok {
		return model.Session{}, repo.ErrSessionNotFound
	}
	return sess, nil
}

func (s *mapSessionStore) DeleteSession(_ context.Context, id string) error {
	delete(s.sessions, id)
	return nil
}

func (s *mapSessionStore) ListSessionsByIdentity(_ context.Context, identityID string) ([]model.Session, error) {
	var out []model.Session
	for _, sess := range s.sessions {
		if sess.IdentityID == identityID {
			out = append(out, sess)
		}
	}
	return out, nil
}

func (s *mapSessionStore) DeleteSessionsByIdentity(_ context.Context, identityID string) error {
	for id, sess := range s.sessions {
		if sess.IdentityID == identityID {
			delete(s.sessions, id)
		}
	}
	return nil
}
