package dao

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

// GetByProviderUID resolves the identity linked to (provider, providerUID) by
// looking up credentials_oauth and reloading the identity row.
// Returns repo.ErrIdentityMissing when no link exists.
func (p *PG) GetByProviderUID(ctx context.Context, provider, providerUID string) (model.Identity, error) {
	val, err := p.db.Model("credentials_oauth").Ctx(ctx).
		Fields("identity_id").
		Where("provider", provider).Where("provider_uid", providerUID).Value()
	if err != nil {
		return model.Identity{}, err
	}
	identityID := val.String()
	if identityID == "" {
		return model.Identity{}, repo.ErrIdentityMissing
	}
	return p.GetByID(ctx, identityID)
}

// CreateOAuthIdentity atomically inserts an identity row, a user_profiles row,
// and a credentials_oauth row inside a single transaction (no password
// credential). Maps a UNIQUE violation on identities.email to repo.ErrEmailTaken
// and on the credentials_oauth PK to repo.ErrProviderUIDTaken.
func (p *PG) CreateOAuthIdentity(ctx context.Context, in repo.NewOAuthIdentityInput) (model.Identity, error) {
	var out model.Identity
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Set context on the transaction so that subsequent Model calls inherit it.
		tx = tx.Ctx(ctx)

		// Insert identity row (with email_verified); capture the DB-generated UUID.
		val, err := tx.GetValue(
			"INSERT INTO identities (email, email_verified) VALUES (?, ?) RETURNING id",
			in.Email, in.EmailVerified,
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
		if _, err := tx.Model("credentials_oauth").Ctx(ctx).Data(g.Map{
			"provider":       in.Provider,
			"provider_uid":   in.ProviderUID,
			"identity_id":    id,
			"email":          in.Email,
			"email_verified": in.EmailVerified,
		}).Insert(); err != nil {
			if isUniqueViolation(err) {
				return repo.ErrProviderUIDTaken
			}
			return err
		}

		// Reload the full identity row (including DB defaults: status, timestamps).
		return tx.Model("identities").Ctx(ctx).Where("id", id).Scan(&out)
	})
	return out, err
}

// LinkOAuthCredential attaches a credentials_oauth row to an existing identity.
// Maps a credentials_oauth PK violation to repo.ErrProviderUIDTaken.
func (p *PG) LinkOAuthCredential(ctx context.Context, identityID, provider, providerUID, email string, emailVerified bool) error {
	_, err := p.db.Model("credentials_oauth").Ctx(ctx).Data(g.Map{
		"provider":       provider,
		"provider_uid":   providerUID,
		"identity_id":    identityID,
		"email":          email,
		"email_verified": emailVerified,
	}).Insert()
	if err != nil {
		if isUniqueViolation(err) {
			return repo.ErrProviderUIDTaken
		}
		return err
	}
	return nil
}

// ListOAuthCredentials returns the identity's bound oauth credentials.
func (p *PG) ListOAuthCredentials(ctx context.Context, identityID string) ([]repo.OAuthCredential, error) {
	var rows []struct {
		Provider string `orm:"provider"`
		Email    string `orm:"email"`
	}
	if err := p.db.Model("credentials_oauth").Ctx(ctx).
		Fields("provider", "email").Where("identity_id", identityID).
		OrderAsc("provider").Scan(&rows); err != nil {
		return nil, err
	}
	out := make([]repo.OAuthCredential, 0, len(rows))
	for _, r := range rows {
		out = append(out, repo.OAuthCredential{Provider: r.Provider, Email: r.Email})
	}
	return out, nil
}

// DeleteOAuthCredential removes the (identityID, provider) credential row.
func (p *PG) DeleteOAuthCredential(ctx context.Context, identityID, provider string) (bool, error) {
	deleted := false
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		locked, err := tx.GetValue(
			`SELECT id FROM identities WHERE id = ? AND status = 'active' FOR UPDATE`,
			identityID,
		)
		if err != nil {
			return err
		}
		if locked.IsNil() || locked.String() == "" {
			return nil
		}
		target, err := tx.GetValue(`
SELECT provider FROM credentials_oauth
WHERE identity_id = ? AND provider = ?
`, identityID, provider)
		if err != nil {
			return err
		}
		if target.IsNil() || target.String() == "" {
			return nil
		}
		alternative, err := tx.GetValue(`
SELECT
    EXISTS (
        SELECT 1 FROM credentials_password WHERE identity_id = ?
    )
    OR EXISTS (
        SELECT 1
        FROM credentials_oauth
        WHERE identity_id = ? AND provider <> ?
    )
    OR EXISTS (
        SELECT 1
        FROM webauthn_credentials
        WHERE identity_id = ? AND status = 'active'
    )
`, identityID, identityID, provider, identityID)
		if err != nil {
			return err
		}
		if !alternative.Bool() {
			return repo.ErrLastCredential
		}
		result, err := tx.Model("credentials_oauth").Ctx(ctx).
			Where("identity_id", identityID).
			Where("provider", provider).
			Delete()
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		deleted = affected > 0
		return nil
	})
	return deleted, err
}

// Compile-time interface assertion.
var _ repo.OAuthRepo = (*PG)(nil)
