package dao

import (
	"context"

	"platform/services/identity/internal/repo"
)

// InsertAudit is a stub that satisfies repo.AuditRepo on *PG.
// TODO(Task 4): replace with real PostgreSQL implementation writing to audit_logs.
func (p *PG) InsertAudit(_ context.Context, _ repo.AuditRow) error {
	return nil
}

// QueryAudit is a stub that satisfies repo.AuditRepo on *PG.
// TODO(Task 4): replace with real PostgreSQL implementation reading from audit_logs.
func (p *PG) QueryAudit(_ context.Context, _ repo.AuditFilter) ([]repo.AuditRow, error) {
	return nil, nil
}

// Compile-time interface assertion.
var _ repo.AuditRepo = (*PG)(nil)
