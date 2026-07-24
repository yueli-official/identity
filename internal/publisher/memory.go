package publisher

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu      sync.Mutex
	records map[string]Attestation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: map[string]Attestation{}}
}

func (store *MemoryStore) GetByIdempotency(
	_ context.Context,
	issuer string,
	subject string,
	key string,
) (Attestation, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, ok := store.records[idempotencyIndex(issuer, subject, key)]
	return value, ok, nil
}

func (store *MemoryStore) PutIfAbsent(
	_ context.Context,
	value Attestation,
) (Attestation, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	index := idempotencyIndex(value.Issuer, value.PublisherSubject, value.IdempotencyKey)
	if existing, ok := store.records[index]; ok {
		return existing, false, nil
	}
	store.records[index] = value
	return value, true, nil
}

func idempotencyIndex(issuer, subject, key string) string {
	return issuer + "\x00" + subject + "\x00" + key
}

var _ Store = (*MemoryStore)(nil)
