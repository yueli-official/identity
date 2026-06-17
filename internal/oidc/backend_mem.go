package oidc

import (
	"context"
	"sync"
)

type memBackend struct {
	mu      sync.Mutex
	generic map[string]Record        // key: kind + "\x00" + signature
	refresh map[string]RefreshRecord // key: signature
}

func newMemBackend() *memBackend {
	return &memBackend{generic: map[string]Record{}, refresh: map[string]RefreshRecord{}}
}

func gkey(kind, sig string) string { return kind + "\x00" + sig }

func (b *memBackend) PutGeneric(_ context.Context, kind, sig string, r Record) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.generic[gkey(kind, sig)] = r
	return nil
}

func (b *memBackend) GetGeneric(_ context.Context, kind, sig string) (Record, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.generic[gkey(kind, sig)]
	if !ok {
		return Record{}, ErrBackendNotFound
	}
	return r, nil
}

func (b *memBackend) DeactivateGeneric(_ context.Context, kind, sig string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r, ok := b.generic[gkey(kind, sig)]; ok {
		r.Active = false
		b.generic[gkey(kind, sig)] = r
	}
	return nil
}

func (b *memBackend) DeleteGeneric(_ context.Context, kind, sig string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.generic, gkey(kind, sig))
	return nil
}

func (b *memBackend) PutRefresh(_ context.Context, sig string, r RefreshRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refresh[sig] = r
	return nil
}

func (b *memBackend) GetRefresh(_ context.Context, sig string) (RefreshRecord, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.refresh[sig]
	if !ok {
		return RefreshRecord{}, ErrBackendNotFound
	}
	if !r.Active {
		return r, ErrBackendInactive
	}
	return r, nil
}

func (b *memBackend) DeactivateRefresh(_ context.Context, sig string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r, ok := b.refresh[sig]; ok {
		r.Active = false
		b.refresh[sig] = r
	}
	return nil
}

func (b *memBackend) DeleteRefresh(_ context.Context, sig string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.refresh, sig)
	return nil
}

func (b *memBackend) revokeWhere(pred func(RefreshRecord) bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for sig, r := range b.refresh {
		if pred(r) {
			delete(b.refresh, sig)
		}
	}
}

func (b *memBackend) RevokeRefreshByRequestID(_ context.Context, reqID string) error {
	b.revokeWhere(func(r RefreshRecord) bool { return r.RequestID == reqID })
	return nil
}

func (b *memBackend) RevokeRefreshBySession(_ context.Context, sessionID string) error {
	b.revokeWhere(func(r RefreshRecord) bool { return r.SessionID == sessionID })
	return nil
}

func (b *memBackend) RevokeRefreshByIdentity(_ context.Context, subject string) error {
	b.revokeWhere(func(r RefreshRecord) bool { return r.Subject == subject })
	return nil
}

// Transactions are no-ops in memory.
func (b *memBackend) BeginTX(ctx context.Context) (context.Context, error) { return ctx, nil }
func (b *memBackend) Commit(context.Context) error                         { return nil }
func (b *memBackend) Rollback(context.Context) error                       { return nil }

var _ Backend = (*memBackend)(nil)
