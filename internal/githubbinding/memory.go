package githubbinding

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MemoryStore struct {
	mu       sync.Mutex
	attempts map[string]Attempt
	bindings map[string]Binding
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		attempts: map[string]Attempt{},
		bindings: map[string]Binding{},
	}
}

func (store *MemoryStore) CreateAttempt(_ context.Context, attempt Attempt) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.attempts[attempt.StateDigest]; exists {
		return ErrInvalidAttempt
	}
	store.attempts[attempt.StateDigest] = attempt
	return nil
}

func (store *MemoryStore) ConsumeAttempt(
	_ context.Context,
	stateDigest string,
	sessionDigest string,
	now time.Time,
) (Attempt, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	attempt, exists := store.attempts[stateDigest]
	if !exists || attempt.ConsumedAt != nil || !attempt.ExpiresAt.After(now) ||
		attempt.SessionDigest != sessionDigest {
		return Attempt{}, ErrInvalidAttempt
	}
	consumedAt := now
	attempt.ConsumedAt = &consumedAt
	store.attempts[stateDigest] = attempt
	return attempt, nil
}

func (store *MemoryStore) Bind(
	_ context.Context,
	identityID string,
	account Account,
	now time.Time,
) (BindResult, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for id, current := range store.bindings {
		if current.ProviderAccountID != account.AccountID || current.Status != StatusActive {
			continue
		}
		if current.IdentityID != identityID {
			return BindResult{}, ErrBindingConflict
		}
		renamed := current.Login != account.Login
		current.Login = account.Login
		current.ProviderNodeID = account.NodeID
		current.AvatarURL = account.AvatarURL
		current.LastVerifiedAt = now
		current.UpdatedAt = now
		store.bindings[id] = current
		return BindResult{Binding: current, Renamed: renamed}, nil
	}
	binding := Binding{
		ID: uuid.NewString(), IdentityID: identityID, Provider: ProviderGitHub,
		ProviderAccountID: account.AccountID, ProviderNodeID: account.NodeID,
		Login: account.Login, AvatarURL: account.AvatarURL, Status: StatusActive,
		VerifiedAt: now, LastVerifiedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	store.bindings[binding.ID] = binding
	return BindResult{Binding: binding, Created: true}, nil
}

func (store *MemoryStore) ListByIdentity(
	_ context.Context,
	identityID string,
) ([]Binding, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]Binding, 0)
	for _, binding := range store.bindings {
		if binding.IdentityID == identityID {
			result = append(result, binding)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].CreatedAt.After(result[right].CreatedAt)
	})
	return result, nil
}

func (store *MemoryStore) FindActiveByAccount(
	_ context.Context,
	accountID string,
) (Binding, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, binding := range store.bindings {
		if binding.ProviderAccountID == accountID && binding.Status == StatusActive {
			return binding, nil
		}
	}
	return Binding{}, ErrBindingInactive
}

func (store *MemoryStore) Unbind(
	_ context.Context,
	identityID string,
	bindingID string,
	now time.Time,
) (Binding, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	binding, exists := store.bindings[bindingID]
	if !exists || binding.IdentityID != identityID || binding.Status != StatusActive {
		return Binding{}, ErrBindingNotFound
	}
	binding.Status = StatusUnbound
	binding.UnboundAt = &now
	binding.UpdatedAt = now
	store.bindings[bindingID] = binding
	return binding, nil
}

func (store *MemoryStore) BlockByAccount(
	_ context.Context,
	accountID string,
	login string,
	now time.Time,
) ([]Binding, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]Binding, 0, 1)
	for id, binding := range store.bindings {
		if binding.ProviderAccountID != accountID || binding.Status != StatusActive {
			continue
		}
		binding.Status = StatusBlocked
		binding.Login = login
		binding.BlockedAt = &now
		binding.UpdatedAt = now
		store.bindings[id] = binding
		result = append(result, binding)
	}
	return result, nil
}

var _ Store = (*MemoryStore)(nil)
