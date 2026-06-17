package repo

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"platform/services/identity/internal/model"
)

// Memory is a hermetic in-memory Store for unit tests and local dev.
type Memory struct {
	mu        sync.Mutex
	byID      map[string]model.Identity
	byEmail   map[string]string // email -> id
	pwHash    map[string]string // id -> hash
	profiles  map[string]model.Profile
	sessions  map[string]model.Session
	failCount map[string]int
	lockUntil map[string]time.Time
	now       func() time.Time
}

func NewMemory() *Memory {
	return &Memory{
		byID: map[string]model.Identity{}, byEmail: map[string]string{},
		pwHash: map[string]string{}, profiles: map[string]model.Profile{},
		sessions: map[string]model.Session{}, failCount: map[string]int{},
		lockUntil: map[string]time.Time{}, now: time.Now,
	}
}

func (m *Memory) CreateIdentityWithProfile(_ context.Context, in NewIdentityInput) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[in.Email]; ok {
		return model.Identity{}, ErrEmailTaken
	}
	id := model.Identity{
		ID: uuid.NewString(), Email: in.Email, Status: model.StatusActive,
		CreatedAt: m.now(), UpdatedAt: m.now(),
	}
	m.byID[id.ID] = id
	m.byEmail[in.Email] = id.ID
	m.pwHash[id.ID] = in.PasswordHash
	m.profiles[id.ID] = model.Profile{IdentityID: id.ID, DisplayName: in.DisplayName, Locale: in.Locale}
	return id, nil
}

func (m *Memory) GetByEmail(_ context.Context, email string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byEmail[email]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return m.byID[id], nil
}

func (m *Memory) GetByID(_ context.Context, id string) (model.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.byID[id]
	if !ok {
		return model.Identity{}, ErrIdentityMissing
	}
	return v, nil
}

func (m *Memory) GetPasswordHash(_ context.Context, identityID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.pwHash[identityID]
	if !ok {
		return "", ErrIdentityMissing
	}
	return h, nil
}

func (m *Memory) CreateSession(_ context.Context, s model.Session, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *Memory) GetSession(_ context.Context, id string) (model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return model.Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (m *Memory) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Memory) ListSessionsByIdentity(_ context.Context, identityID string) ([]model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Session
	for _, s := range m.sessions {
		if s.IdentityID == identityID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *Memory) DeleteSessionsByIdentity(_ context.Context, identityID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		if s.IdentityID == identityID {
			delete(m.sessions, k)
		}
	}
	return nil
}

func (m *Memory) Locked(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok := m.lockUntil[key]
	return ok && m.now().Before(until), nil
}

func (m *Memory) RecordFailure(_ context.Context, key string, _, lockDur time.Duration, max int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount[key]++
	if m.failCount[key] >= max {
		m.lockUntil[key] = m.now().Add(lockDur)
	}
	return nil
}

func (m *Memory) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failCount, key)
	delete(m.lockUntil, key)
	return nil
}

// Compile-time guarantee Memory satisfies the full Store surface.
var _ Store = (*Memory)(nil)
