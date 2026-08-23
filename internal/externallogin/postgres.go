package externallogin

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
)

const providerTable = "external_login_providers"

type PostgresStore struct {
	db gdb.DB
}

func NewPostgresStore(db gdb.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (store *PostgresStore) Get(ctx context.Context, key string) (Config, error) {
	var config Config
	if err := store.db.Model(providerTable).Ctx(ctx).Where("key", key).Scan(&config); err != nil {
		return Config{}, err
	}
	if config.Key == "" {
		return Config{}, ErrNotFound
	}
	return config, nil
}

func (store *PostgresStore) List(ctx context.Context) ([]Config, error) {
	configs := []Config{}
	if err := store.db.Model(providerTable).Ctx(ctx).OrderAsc("key").Scan(&configs); err != nil {
		return nil, err
	}
	return configs, nil
}

func (store *PostgresStore) Upsert(ctx context.Context, config Config, actorID string) (Config, error) {
	err := store.db.Transaction(ctx, func(ctx context.Context, transaction gdb.TX) error {
		if _, err := transaction.Exec(`
INSERT INTO external_login_providers (
    key, client_id, client_secret_cipher, secret_version, enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, NOW(), NOW())
ON CONFLICT (key) DO UPDATE SET
    client_id = EXCLUDED.client_id,
    client_secret_cipher = EXCLUDED.client_secret_cipher,
    secret_version = EXCLUDED.secret_version,
    enabled = EXCLUDED.enabled,
    updated_at = NOW()
`, config.Key, config.ClientID, config.ClientSecretCipher, config.SecretVersion, config.Enabled); err != nil {
			return err
		}
		_, err := transaction.Exec(`
INSERT INTO external_login_provider_events (provider_key, actor_id, event, metadata)
VALUES (?, ?, 'configured', jsonb_build_object('enabled', ?, 'secretVersion', ?))
`, config.Key, actorID, config.Enabled, config.SecretVersion)
		return err
	})
	if err != nil {
		return Config{}, err
	}
	return store.Get(ctx, config.Key)
}

func (store *PostgresStore) UpdateHealth(ctx context.Context, key string, healthy bool, message string, checkedAt time.Time, actorID string) error {
	var affected int64
	err := store.db.Transaction(ctx, func(ctx context.Context, transaction gdb.TX) error {
		result, err := transaction.Model(providerTable).Ctx(ctx).Where("key", key).Data(map[string]any{
			"last_health_ok":         healthy,
			"last_health_checked_at": checkedAt,
			"last_health_error":      message,
			"updated_at":             checkedAt,
		}).Update()
		if err != nil {
			return err
		}
		affected, _ = result.RowsAffected()
		if affected == 0 {
			return ErrNotFound
		}
		_, err = transaction.Exec(`
INSERT INTO external_login_provider_events (provider_key, actor_id, event, metadata)
VALUES (?, ?, 'health_checked', jsonb_build_object('healthy', ?))
`, key, actorID, healthy)
		return err
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

var _ Store = (*PostgresStore)(nil)
