package stepup

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// PostgreSQLReplayStore atomically consumes JTIs in a consumer-owned table:
//
// CREATE TABLE step_up_proof_uses (
//
//	jti UUID PRIMARY KEY,
//	expires_at TIMESTAMPTZ NOT NULL,
//	consumed_at TIMESTAMPTZ NOT NULL DEFAULT now()
//
// );
//
// Consumers may delete expired rows asynchronously.
type PostgreSQLReplayStore struct {
	DB *sql.DB
}

func (store PostgreSQLReplayStore) Consume(
	ctx context.Context,
	jti string,
	expiresAt time.Time,
) (bool, error) {
	result, err := store.DB.ExecContext(ctx, `
INSERT INTO step_up_proof_uses(jti, expires_at)
VALUES ($1, $2)
ON CONFLICT(jti) DO NOTHING
`, jti, expiresAt)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

type MemoryReplayStore struct {
	mu   sync.Mutex
	used map[string]time.Time
}

func NewMemoryReplayStore() *MemoryReplayStore {
	return &MemoryReplayStore{used: map[string]time.Time{}}
}

func (store *MemoryReplayStore) Consume(
	_ context.Context,
	jti string,
	expiresAt time.Time,
) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.used[jti]; exists {
		return false, nil
	}
	store.used[jti] = expiresAt
	return true, nil
}
