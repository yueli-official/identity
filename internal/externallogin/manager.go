package externallogin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yueli-official/identity/internal/oauthlogin"
)

var (
	ErrNotFound    = errors.New("external login provider not found")
	ErrUnavailable = errors.New("external login provider unavailable")
	ErrInvalid     = errors.New("invalid external login provider configuration")
)

type Config struct {
	Key                string     `orm:"key"`
	ClientID           string     `orm:"client_id"`
	ClientSecretCipher string     `orm:"client_secret_cipher"`
	SecretVersion      int        `orm:"secret_version"`
	Enabled            bool       `orm:"enabled"`
	LastHealthOK       *bool      `orm:"last_health_ok"`
	LastHealthChecked  *time.Time `orm:"last_health_checked_at"`
	LastHealthError    string     `orm:"last_health_error"`
	CreatedAt          time.Time  `orm:"created_at"`
	UpdatedAt          time.Time  `orm:"updated_at"`
}

type Store interface {
	Get(context.Context, string) (Config, error)
	List(context.Context) ([]Config, error)
	Upsert(context.Context, Config, string) (Config, error)
	UpdateHealth(context.Context, string, bool, string, time.Time, string) error
}

type Definition struct {
	Key                string
	Label              string
	RegistrationPolicy oauthlogin.RegistrationPolicy
	Build              func(clientID, clientSecret, redirectURL string) oauthlogin.Provider
}

var catalog = map[string]Definition{
	"google": {
		Key: "google", Label: "Google",
		RegistrationPolicy: oauthlogin.RegistrationVerifiedEmail,
		Build: func(clientID, clientSecret, redirectURL string) oauthlogin.Provider {
			return oauthlogin.NewGoogle(clientID, clientSecret, redirectURL)
		},
	},
	"qq": {
		Key: "qq", Label: "QQ",
		RegistrationPolicy: oauthlogin.RegistrationExistingOnly,
		Build: func(clientID, clientSecret, redirectURL string) oauthlogin.Provider {
			return oauthlogin.NewQQ(clientID, clientSecret, redirectURL)
		},
	},
}

type View struct {
	Key                string                        `json:"key"`
	Label              string                        `json:"label"`
	RegistrationPolicy oauthlogin.RegistrationPolicy `json:"registrationPolicy"`
	Configured         bool                          `json:"configured"`
	Enabled            bool                          `json:"enabled"`
	ClientID           string                        `json:"clientId"`
	RedirectURL        string                        `json:"redirectUrl"`
	SecretVersion      int                           `json:"secretVersion"`
	LastHealthOK       *bool                         `json:"lastHealthOk,omitempty"`
	LastHealthChecked  *time.Time                    `json:"lastHealthCheckedAt,omitempty"`
	LastHealthError    string                        `json:"lastHealthError,omitempty"`
	UpdatedAt          *time.Time                    `json:"updatedAt,omitempty"`
}

type SaveInput struct {
	ActorID      string
	Key          string
	ClientID     string
	ClientSecret string
	Enabled      bool
}

type BootstrapInput struct {
	Key          string
	ClientID     string
	ClientSecret string
	Enabled      bool
}

type Manager struct {
	store          Store
	secrets        *SecretBox
	accountBaseURL string
	now            func() time.Time
}

func New(store Store, secretMaterial, accountBaseURL string) (*Manager, error) {
	if store == nil {
		return nil, errors.New("external login store is required")
	}
	secrets, err := NewSecretBox(secretMaterial)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(accountBaseURL), "/")
	if base == "" {
		return nil, errors.New("account base URL is required")
	}
	return &Manager{store: store, secrets: secrets, accountBaseURL: base, now: time.Now}, nil
}

func (manager *Manager) RedirectURL(key string) string {
	return manager.accountBaseURL + "/api/v1/auth/oauth/" + key + "/callback"
}

func (manager *Manager) List(ctx context.Context) ([]View, error) {
	configs, err := manager.store.List(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]Config, len(configs))
	for _, config := range configs {
		byKey[config.Key] = config
	}
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	views := make([]View, 0, len(keys))
	for _, key := range keys {
		config, configured := byKey[key]
		views = append(views, manager.view(catalog[key], config, configured))
	}
	return views, nil
}

func (manager *Manager) Public(ctx context.Context) ([]View, error) {
	views, err := manager.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]View, 0, len(views))
	for _, view := range views {
		if view.Configured && view.Enabled {
			result = append(result, view)
		}
	}
	return result, nil
}

func (manager *Manager) Save(ctx context.Context, input SaveInput) (View, error) {
	key := strings.ToLower(strings.TrimSpace(input.Key))
	definition, ok := catalog[key]
	if !ok {
		return View{}, fmt.Errorf("%w: unsupported provider %q", ErrInvalid, key)
	}
	clientID := strings.TrimSpace(input.ClientID)
	if clientID == "" {
		return View{}, fmt.Errorf("%w: client ID is required", ErrInvalid)
	}
	existing, err := manager.store.Get(ctx, key)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return View{}, err
	}
	secretCipher := existing.ClientSecretCipher
	secretVersion := existing.SecretVersion
	if strings.TrimSpace(input.ClientSecret) != "" {
		secretCipher, err = manager.secrets.Encrypt(key, strings.TrimSpace(input.ClientSecret))
		if err != nil {
			return View{}, err
		}
		secretVersion++
	}
	if secretCipher == "" {
		return View{}, fmt.Errorf("%w: client secret is required", ErrInvalid)
	}
	actorID := strings.TrimSpace(input.ActorID)
	if actorID == "" {
		return View{}, fmt.Errorf("%w: actor is required", ErrInvalid)
	}
	config, err := manager.store.Upsert(ctx, Config{
		Key: key, ClientID: clientID, ClientSecretCipher: secretCipher,
		SecretVersion: secretVersion, Enabled: input.Enabled,
	}, actorID)
	if err != nil {
		return View{}, err
	}
	return manager.view(definition, config, true), nil
}

func (manager *Manager) Bootstrap(ctx context.Context, inputs ...BootstrapInput) error {
	for _, input := range inputs {
		if strings.TrimSpace(input.ClientID) == "" || strings.TrimSpace(input.ClientSecret) == "" {
			continue
		}
		if _, err := manager.store.Get(ctx, input.Key); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if _, err := manager.Save(ctx, SaveInput{
			ActorID: "bootstrap",
			Key:     input.Key, ClientID: input.ClientID,
			ClientSecret: input.ClientSecret, Enabled: input.Enabled,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (manager *Manager) Resolve(ctx context.Context, key string) (oauthlogin.Provider, oauthlogin.RegistrationPolicy, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	definition, ok := catalog[key]
	if !ok {
		return nil, "", ErrUnavailable
	}
	config, err := manager.store.Get(ctx, key)
	if err != nil || !config.Enabled || strings.TrimSpace(config.ClientID) == "" || config.ClientSecretCipher == "" {
		return nil, definition.RegistrationPolicy, ErrUnavailable
	}
	secret, err := manager.secrets.Decrypt(key, config.ClientSecretCipher)
	if err != nil {
		return nil, definition.RegistrationPolicy, err
	}
	return definition.Build(config.ClientID, secret, manager.RedirectURL(key)), definition.RegistrationPolicy, nil
}

func (manager *Manager) CheckHealth(ctx context.Context, key, actorID string) error {
	provider, _, err := manager.Resolve(ctx, key)
	if err != nil {
		return err
	}
	checker, ok := provider.(oauthlogin.HealthChecker)
	if !ok {
		return nil
	}
	checkedAt := manager.now().UTC()
	err = checker.CheckHealth(ctx)
	message := ""
	if err != nil {
		message = err.Error()
	}
	if storeErr := manager.store.UpdateHealth(ctx, key, err == nil, message, checkedAt, actorID); storeErr != nil {
		return storeErr
	}
	return err
}

func (manager *Manager) view(definition Definition, config Config, configured bool) View {
	view := View{
		Key: definition.Key, Label: definition.Label,
		RegistrationPolicy: definition.RegistrationPolicy,
		Configured:         configured && config.ClientID != "" && config.ClientSecretCipher != "",
		Enabled:            config.Enabled, ClientID: config.ClientID,
		RedirectURL: manager.RedirectURL(definition.Key), SecretVersion: config.SecretVersion,
		LastHealthOK: config.LastHealthOK, LastHealthChecked: config.LastHealthChecked,
		LastHealthError: config.LastHealthError,
	}
	if configured && !config.UpdatedAt.IsZero() {
		updated := config.UpdatedAt
		view.UpdatedAt = &updated
	}
	return view
}
