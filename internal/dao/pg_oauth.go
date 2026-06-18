package dao

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
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

// Compile-time interface assertion.
var _ repo.OAuthRepo = (*PG)(nil)
