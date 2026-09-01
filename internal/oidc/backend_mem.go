package oidc

import (
	"context"
	"sync"
	"time"
)

type memBackend struct {
	mu      sync.Mutex
	generic map[string]Record        // key: kind + "\x00" + signature
	refresh map[string]RefreshRecord // key: signature
	replays map[string]RefreshReplayReceipt
}

func newMemBackend() *memBackend {
	return &memBackend{generic: map[string]Record{}, refresh: map[string]RefreshRecord{}, replays: map[string]RefreshReplayReceipt{}}
}

func (b *memBackend) PutRefreshReplay(_ context.Context, receipt RefreshReplayReceipt) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, value := range b.replays {
		if !value.ExpiresAt.After(time.Now().UTC()) {
			delete(b.replays, key)
		}
	}
	b.replays[receipt.KeyDigest] = receipt
	return nil
}
func (b *memBackend) GetRefreshReplay(_ context.Context, key, client string, now time.Time) (RefreshReplayReceipt, bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, ok := b.replays[key]
	if !ok || value.ClientID != client || !value.ExpiresAt.After(now) {
		return RefreshReplayReceipt{}, false, nil
	}
	active := false
	for _, refresh := range b.refresh {
		if refresh.Active && refresh.RequestID == value.RequestID {
			active = true
			break
		}
	}
	if !active {
		return RefreshReplayReceipt{}, false, nil
	}
	return value, true, nil
}

// NewMemBackend is the exported constructor for the in-memory Backend. It lets
// external packages (cmd wiring, oidc_test black-box tests) build a Store before
// the PG backend lands (Task 9). Sessions/refresh tokens are NOT durable.
func NewMemBackend() Backend { return newMemBackend() }

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
	if sessionID == "" {
		return nil // never mass-revoke rows with an empty session_id (parity with pgBackend)
	}
	b.revokeWhere(func(r RefreshRecord) bool { return r.SessionID == sessionID })
	return nil
}

func (b *memBackend) RevokeRefreshBySubject(_ context.Context, subject string) error {
	b.revokeWhere(func(r RefreshRecord) bool { return r.Subject == subject })
	return nil
}

// Transactions are no-ops in memory.
func (b *memBackend) BeginTX(ctx context.Context) (context.Context, error) { return ctx, nil }
func (b *memBackend) Commit(context.Context) error                         { return nil }
func (b *memBackend) Rollback(context.Context) error                       { return nil }

var _ Backend = (*memBackend)(nil)
