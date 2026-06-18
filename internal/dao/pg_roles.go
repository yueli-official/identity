package dao

import (
	"context"

	"platform/services/identity/internal/repo"
)

// GetRoles returns the identity's granted role slugs, sorted by slug ascending
// (empty, len 0, when none). Array reads a single column across rows; gdb.Scan
// does not support a []string scalar destination, so we use Array("role_slug")
// and unwrap the []*gvar.Var into []string.
func (p *PG) GetRoles(ctx context.Context, identityID string) ([]string, error) {
	rows, err := p.db.Model("identity_roles").Ctx(ctx).
		Where("identity_id", identityID).Order("role_slug ASC").
		Fields("role_slug").Array()
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(rows))
	for _, v := range rows {
		slugs = append(slugs, v.String())
	}
	return slugs, nil
}

// GrantRole grants a catalog role to the identity. It validates the slug against
// the roles catalog first (returning repo.ErrUnknownRole when absent), then
// inserts the (identity_id, role_slug) pair. The insert uses a raw
// ON CONFLICT DO NOTHING so re-granting an existing pair is an idempotent no-op
// (the join row has no non-key columns to update, so DO NOTHING is the correct
// upsert here).
func (p *PG) GrantRole(ctx context.Context, identityID, slug string) error {
	ok, err := p.db.Model("roles").Ctx(ctx).Where("slug", slug).Exist()
	if err != nil {
		return err
	}
	if !ok {
		return repo.ErrUnknownRole
	}
	_, err = p.db.Exec(ctx,
		"INSERT INTO identity_roles (identity_id, role_slug) VALUES (?, ?) ON CONFLICT DO NOTHING",
		identityID, slug,
	)
	return err
}

// RevokeRole removes a role grant from the identity. Revoking a pair the
// identity does not have is a no-op (Delete reports zero rows affected, no error).
func (p *PG) RevokeRole(ctx context.Context, identityID, slug string) error {
	_, err := p.db.Model("identity_roles").Ctx(ctx).
		Where("identity_id", identityID).Where("role_slug", slug).Delete()
	return err
}

// Compile-time interface assertion.
var _ repo.RoleRepo = (*PG)(nil)
