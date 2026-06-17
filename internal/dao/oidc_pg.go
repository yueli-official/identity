package dao

import (
	"context"

	"github.com/lib/pq"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// GetClient loads a registered OIDC client from oidc_clients.
// The text[] columns (redirect_uris, grant_types, response_types, scopes) are
// decoded with pq.Array because gdb.Scan does not automatically convert a
// PostgreSQL text[] column into []string.
func (p *PG) GetClient(ctx context.Context, id string) (model.OIDCClient, error) {
	row, err := p.db.GetOne(ctx,
		"SELECT id, public, secret_hash, redirect_uris, grant_types, response_types, scopes FROM oidc_clients WHERE id = $1",
		id,
	)
	if err != nil {
		return model.OIDCClient{}, err
	}
	if len(row) == 0 {
		return model.OIDCClient{}, repo.ErrClientNotFound
	}

	var redirectURIs, grantTypes, responseTypes, scopes pq.StringArray
	if v, ok := row["redirect_uris"]; ok && v != nil {
		if err := pq.Array(&redirectURIs).Scan(v.Val()); err != nil {
			return model.OIDCClient{}, err
		}
	}
	if v, ok := row["grant_types"]; ok && v != nil {
		if err := pq.Array(&grantTypes).Scan(v.Val()); err != nil {
			return model.OIDCClient{}, err
		}
	}
	if v, ok := row["response_types"]; ok && v != nil {
		if err := pq.Array(&responseTypes).Scan(v.Val()); err != nil {
			return model.OIDCClient{}, err
		}
	}
	if v, ok := row["scopes"]; ok && v != nil {
		if err := pq.Array(&scopes).Scan(v.Val()); err != nil {
			return model.OIDCClient{}, err
		}
	}

	return model.OIDCClient{
		ID:            row["id"].String(),
		Public:        row["public"].Bool(),
		SecretHash:    row["secret_hash"].String(),
		RedirectURIs:  []string(redirectURIs),
		GrantTypes:    []string(grantTypes),
		ResponseTypes: []string(responseTypes),
		Scopes:        []string(scopes),
	}, nil
}

// GetActiveKey returns the currently active signing key.
func (p *PG) GetActiveKey(ctx context.Context) (model.SigningKey, error) {
	var k model.SigningKey
	if err := p.db.Model("oidc_signing_keys").Ctx(ctx).Where("status", "active").Scan(&k); err != nil {
		return model.SigningKey{}, err
	}
	if k.KID == "" {
		return model.SigningKey{}, repo.ErrNoActiveKey
	}
	return k, nil
}

// InsertKey persists a new signing key.
func (p *PG) InsertKey(ctx context.Context, k model.SigningKey) error {
	_, err := p.db.Model("oidc_signing_keys").Ctx(ctx).Data(map[string]interface{}{
		"kid":         k.KID,
		"alg":         k.Alg,
		"private_pem": k.PrivatePEM,
		"public_pem":  k.PublicPEM,
		"status":      string(k.Status),
	}).Insert()
	return err
}

// ListPublicKeys returns all active and retired signing keys (used to build JWKS).
func (p *PG) ListPublicKeys(ctx context.Context) ([]model.SigningKey, error) {
	var ks []model.SigningKey
	if err := p.db.Model("oidc_signing_keys").Ctx(ctx).
		WhereIn("status", []string{"active", "retired"}).Scan(&ks); err != nil {
		return nil, err
	}
	return ks, nil
}

// Compile-time assertions: *PG must satisfy both OIDC repo interfaces.
var _ repo.ClientRepo = (*PG)(nil)
var _ repo.SigningKeyRepo = (*PG)(nil)
