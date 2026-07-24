package identitymaintenance

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	DefaultRetention = 24 * time.Hour
	DefaultBatchSize = 500
)

type Result struct {
	Ceremonies   int64
	Transactions int64
	PendingTOTP  int64
	ProofUses    int64
}

type SecurityRetention struct {
	DB        *sql.DB
	Retention time.Duration
	BatchSize int
	Clock     func() time.Time
}

func (cleaner SecurityRetention) RunOnce(ctx context.Context) (Result, error) {
	if cleaner.DB == nil {
		return Result{}, fmt.Errorf("identity maintenance database is required")
	}
	retention := cleaner.Retention
	if retention <= 0 {
		retention = DefaultRetention
	}
	batchSize := cleaner.BatchSize
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	if batchSize > 10_000 {
		return Result{}, fmt.Errorf("identity maintenance batch size must not exceed 10000")
	}
	now := time.Now().UTC()
	if cleaner.Clock != nil {
		now = cleaner.Clock().UTC()
	}
	cutoff := now.Add(-retention)

	result := Result{}
	var err error
	if result.Ceremonies, err = deleteBatch(
		ctx, cleaner.DB, "authentication_ceremonies", "id", "expires_at", cutoff, batchSize,
	); err != nil {
		return result, fmt.Errorf("delete authentication ceremonies: %w", err)
	}
	if result.Transactions, err = deleteBatch(
		ctx, cleaner.DB, "authentication_transactions", "id", "expires_at", cutoff, batchSize,
	); err != nil {
		return result, fmt.Errorf("delete authentication transactions: %w", err)
	}
	if result.PendingTOTP, err = deletePendingTOTP(ctx, cleaner.DB, cutoff, batchSize); err != nil {
		return result, fmt.Errorf("delete pending TOTP enrollments: %w", err)
	}
	if result.ProofUses, err = deleteBatch(
		ctx, cleaner.DB, "step_up_proof_uses", "jti", "expires_at", now, batchSize,
	); err != nil {
		return result, fmt.Errorf("delete step-up proof uses: %w", err)
	}
	return result, nil
}

func deleteBatch(
	ctx context.Context,
	db *sql.DB,
	table string,
	keyColumn string,
	expiryColumn string,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	query := fmt.Sprintf(`
WITH doomed AS (
    SELECT %s
    FROM %s
    WHERE %s < $1
    ORDER BY %s
    LIMIT $2
)
DELETE FROM %s AS target
USING doomed
WHERE target.%s = doomed.%s
`, keyColumn, table, expiryColumn, expiryColumn, table, keyColumn, keyColumn)
	result, err := db.ExecContext(ctx, query, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func deletePendingTOTP(
	ctx context.Context,
	db *sql.DB,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	result, err := db.ExecContext(ctx, `
WITH doomed AS (
    SELECT id
    FROM totp_authenticators
    WHERE status = 'pending' AND enrollment_expires_at < $1
    ORDER BY enrollment_expires_at
    LIMIT $2
)
DELETE FROM totp_authenticators AS target
USING doomed
WHERE target.id = doomed.id
`, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
