package dao

import (
	"context"

	"platform/services/identity/internal/repo"
)

// TODO(Task 3): replace these stubs with the real PG-backed RoleRepo impl
// (roles / identity_roles tables, migration 0006). They exist only so *PG
// satisfies repo.Store (via repo.NewComposite) and the build stays green until
// Task 3 lands. They must not be relied on in production.

// GetRoles is a Task-2 placeholder; see the TODO above.
func (p *PG) GetRoles(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// GrantRole is a Task-2 placeholder; see the TODO above.
func (p *PG) GrantRole(_ context.Context, _, _ string) error {
	return nil
}

// RevokeRole is a Task-2 placeholder; see the TODO above.
func (p *PG) RevokeRole(_ context.Context, _, _ string) error {
	return nil
}

// Compile-time interface assertion (the real impl in Task 3 keeps this).
var _ repo.RoleRepo = (*PG)(nil)
