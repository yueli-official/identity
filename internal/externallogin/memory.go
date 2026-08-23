package externallogin

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.Mutex
	configs map[string]Config
	now     func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{configs: map[string]Config{}, now: time.Now}
}

func (store *MemoryStore) Get(_ context.Context, key string) (Config, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	config, ok := store.configs[key]
	if !ok {
		return Config{}, ErrNotFound
	}
	return config, nil
}

func (store *MemoryStore) List(context.Context) ([]Config, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	configs := make([]Config, 0, len(store.configs))
	for _, config := range store.configs {
		configs = append(configs, config)
	}
	sort.Slice(configs, func(left, right int) bool { return configs[left].Key < configs[right].Key })
	return configs, nil
}

func (store *MemoryStore) Upsert(_ context.Context, config Config, _ string) (Config, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now().UTC()
	existing, ok := store.configs[config.Key]
	if ok {
		config.CreatedAt = existing.CreatedAt
		config.LastHealthOK = existing.LastHealthOK
		config.LastHealthChecked = existing.LastHealthChecked
		config.LastHealthError = existing.LastHealthError
	} else {
		config.CreatedAt = now
	}
	config.UpdatedAt = now
	store.configs[config.Key] = config
	return config, nil
}

func (store *MemoryStore) UpdateHealth(_ context.Context, key string, healthy bool, message string, checkedAt time.Time, _ string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	config, ok := store.configs[key]
	if !ok {
		return ErrNotFound
	}
	config.LastHealthOK = &healthy
	config.LastHealthChecked = &checkedAt
	config.LastHealthError = message
	config.UpdatedAt = checkedAt
	store.configs[key] = config
	return nil
}
