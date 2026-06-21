package dao

import (
	"context"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// GetClient loads a registered OIDC client from oidc_clients. gdb's pgsql driver
// already decodes the text[] columns (redirect_uris, grant_types, response_types,
// scopes) into []string, so they are read directly via gvar's .Strings(). The
// earlier pq.Array path double-decoded and failed on current drivers with
// "cannot convert []string to pq.StringArray", which made every client load fail.
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

	return model.OIDCClient{
		ID:            row["id"].String(),
		Public:        row["public"].Bool(),
		SecretHash:    row["secret_hash"].String(),
		RedirectURIs:  row["redirect_uris"].Strings(),
		GrantTypes:    row["grant_types"].Strings(),
		ResponseTypes: row["response_types"].Strings(),
		Scopes:        row["scopes"].Strings(),
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
