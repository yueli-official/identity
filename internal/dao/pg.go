// Package dao implements the PostgreSQL-backed IdentityRepo (gdb). The atomic
// create runs in a gdb transaction (identity + profile + password credential).
package dao

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// PG is a PostgreSQL-backed implementation of repo.IdentityRepo using gdb.
type PG struct {
	db gdb.DB
}

// NewPG creates a PG repo backed by the given gdb.DB connection.
func NewPG(db gdb.DB) *PG { return &PG{db: db} }

// CreateIdentityWithProfile atomically inserts an identity row, a user_profiles
// row, and a credentials_password row inside a single transaction.
// It returns repo.ErrEmailTaken when a UNIQUE constraint on identities.email fires.
func (p *PG) CreateIdentityWithProfile(ctx context.Context, in repo.NewIdentityInput) (model.Identity, error) {
	var out model.Identity
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Set context on the transaction so that subsequent Model calls inherit it.
		tx = tx.Ctx(ctx)

		// Insert identity row; capture the DB-generated UUID via RETURNING.
		// TX.GetValue does not accept a ctx parameter (it uses tx.ctx set above).
		val, err := tx.GetValue(
			"INSERT INTO identities (email) VALUES (?) RETURNING id", in.Email,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return repo.ErrEmailTaken
			}
			return err
		}
		id := val.String()

		if _, err := tx.Model("user_profiles").Ctx(ctx).Data(g.Map{
			"identity_id":  id,
			"display_name": in.DisplayName,
			"locale":       orDefault(in.Locale, "zh-CN"),
		}).Insert(); err != nil {
			return err
		}
		if _, err := tx.Model("credentials_password").Ctx(ctx).Data(g.Map{
			"identity_id":   id,
			"password_hash": in.PasswordHash,
		}).Insert(); err != nil {
			return err
		}

		// Reload the full identity row (including DB defaults: status, timestamps).
		return tx.Model("identities").Ctx(ctx).Where("id", id).Scan(&out)
	})
	return out, err
}

// GetByEmail returns the non-deleted identity with the given email address.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetByEmail(ctx context.Context, email string) (model.Identity, error) {
	var out model.Identity
	err := p.db.Model("identities").Ctx(ctx).
		Where("email", email).Where("status <>", string(model.StatusDeleted)).Scan(&out)
	if err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

// GetByID returns the identity with the given UUID.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetByID(ctx context.Context, id string) (model.Identity, error) {
	var out model.Identity
	if err := p.db.Model("identities").Ctx(ctx).Where("id", id).Scan(&out); err != nil {
		return model.Identity{}, err
	}
	if out.ID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return out, nil
}

// GetPasswordHash returns the bcrypt password hash stored for the given identity.
// Model.Scan does not support scalar destinations; we use Value() instead.
func (p *PG) GetPasswordHash(ctx context.Context, identityID string) (string, error) {
	val, err := p.db.Model("credentials_password").Ctx(ctx).
		Fields("password_hash").Where("identity_id", identityID).Value()
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// GetProfile returns the user_profiles row for an identity.
// Returns repo.ErrIdentityMissing when absent.
func (p *PG) GetProfile(ctx context.Context, identityID string) (model.Profile, error) {
	var out model.Profile
	if err := p.db.Model("user_profiles").Ctx(ctx).Where("identity_id", identityID).Scan(&out); err != nil {
		return model.Profile{}, err
	}
	if out.IdentityID == "" {
		return model.Profile{}, repo.ErrIdentityMissing
	}
	return out, nil
}

// orDefault returns v if non-blank, otherwise d.
func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

// nilIfEmpty returns nil when s is blank (empty or whitespace-only), otherwise
// the ORIGINAL string s. Used to map empty-string sentinel values to SQL NULL
// for UUID columns and to keep nullable TEXT columns NULL rather than storing
// whitespace-only junk.
func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// Compile-time interface assertion.
var _ repo.IdentityRepo = (*PG)(nil)
