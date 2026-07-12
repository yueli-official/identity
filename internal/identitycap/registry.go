package identitycap

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"platform/gokit/capability"
)

type HealthChecker interface {
	CheckHealth(context.Context) error
}

type HealthCheckFunc func(context.Context) error

func (check HealthCheckFunc) CheckHealth(ctx context.Context) error { return check(ctx) }

type Registration struct {
	Key            string
	Adapter        string
	Mode           string
	Registered     bool
	Enabled        bool
	CapabilityKeys []string
	Operations     []string
	RequiredConfig []capability.ConfigField
	Checker        HealthChecker
	InitialHealth  capability.Health
}

type entry struct {
	registration Registration
	health       capability.Health
	checkedAt    *time.Time
}

type Registry struct {
	mu          sync.Mutex
	providers   map[string]*entry
	service     capability.ServiceMetadata
	snapshot    *capability.Snapshot
	generatedAt time.Time
}

func New(registrations ...Registration) (*Registry, error) {
	registry := &Registry{providers: map[string]*entry{}}
	enabled := map[string]string{}
	for _, registration := range registrations {
		registration = cloneRegistration(registration)
		registration.Key = strings.TrimSpace(registration.Key)
		registration.Adapter = strings.TrimSpace(registration.Adapter)
		if registration.Key == "" || registration.Adapter == "" || len(registration.CapabilityKeys) == 0 {
			return nil, fmt.Errorf("identity provider key, adapter, and capability keys are required")
		}
		if _, exists := registry.providers[registration.Key]; exists {
			return nil, fmt.Errorf("duplicate identity provider %q", registration.Key)
		}
		if registration.Enabled && !registration.Registered {
			return nil, fmt.Errorf("enabled identity provider %q must be registered", registration.Key)
		}
		if registration.Enabled {
			for _, capabilityKey := range registration.CapabilityKeys {
				if current := enabled[capabilityKey]; current != "" {
					return nil, fmt.Errorf("identity capability %q has multiple enabled providers", capabilityKey)
				}
				enabled[capabilityKey] = registration.Key
			}
		}
		if registration.InitialHealth == "" {
			registration.InitialHealth = capability.HealthUnknown
		}
		registry.providers[registration.Key] = &entry{registration: registration, health: registration.InitialHealth}
	}
	return registry, nil
}

func (registry *Registry) Snapshot(service capability.ServiceMetadata, generatedAt time.Time) (*capability.Snapshot, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.snapshot != nil && registry.service == service {
		return registry.snapshot, nil
	}
	registry.service = service
	return registry.rebuildLocked(generatedAt)
}

func (registry *Registry) CheckHealth(ctx context.Context, key string) error {
	registry.mu.Lock()
	provider := registry.providers[strings.TrimSpace(key)]
	registry.mu.Unlock()
	if provider == nil {
		return fmt.Errorf("identity provider %q not found", key)
	}
	var probeErr error
	if !provider.registration.Registered {
		probeErr = fmt.Errorf("identity provider %q is not registered", key)
	} else if configurationFrom(provider.registration.RequiredConfig) != capability.ConfigurationComplete {
		probeErr = fmt.Errorf("identity provider %q configuration is incomplete", key)
	} else if provider.registration.Checker == nil {
		probeErr = fmt.Errorf("identity provider %q has no health checker", key)
	} else {
		probeErr = provider.registration.Checker.CheckHealth(ctx)
	}
	now := time.Now().UTC()
	registry.mu.Lock()
	provider.checkedAt = &now
	provider.health = capability.HealthHealthy
	if probeErr != nil {
		provider.health = capability.HealthUnhealthy
	}
	registry.snapshot = nil
	if registry.service.Name != "" {
		_, _ = registry.rebuildLocked(now)
	}
	registry.mu.Unlock()
	return probeErr
}

func (registry *Registry) rebuildLocked(generatedAt time.Time) (*capability.Snapshot, error) {
	generatedAt = generatedAt.UTC()
	if !registry.generatedAt.IsZero() && !generatedAt.After(registry.generatedAt) {
		generatedAt = registry.generatedAt.Add(time.Nanosecond)
	}
	keys := make([]string, 0, len(registry.providers))
	for key := range registry.providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	providers := make([]capability.Provider, 0, len(keys))
	for _, key := range keys {
		providers = append(providers, providerView(registry.providers[key]))
	}

	capabilities := coreCapabilities()
	for index := range capabilities {
		selected := registry.selectedProvider(capabilities[index].Key)
		if selected == nil {
			continue
		}
		view := providerView(selected)
		capabilities[index].Configuration = view.Configuration
		capabilities[index].Enablement = view.Enablement
		capabilities[index].Health = view.Health
		capabilities[index].RequiredConfig = view.RequiredConfig
		capabilities[index].LastCheckedAt = view.LastCheckedAt
		if view.Registered {
			capabilities[index].Adapter = view.Adapter
			capabilities[index].ProviderInstance = view.Key
		}
	}
	snapshot, err := capability.NewSnapshot(capability.Manifest{
		Service: registry.service, GeneratedAt: generatedAt,
		Redaction:    capability.RedactionMetadata{Policy: "presence-only", Version: "1"},
		Capabilities: capabilities, Providers: providers,
		Links: []capability.Link{{Rel: "health", Href: "/healthz"}, {Rel: "ready", Href: "/readyz"}, {Rel: "users", Href: "/api/v1/admin/identities"}},
	})
	if err != nil {
		return nil, err
	}
	registry.snapshot, registry.generatedAt = snapshot, generatedAt
	return snapshot, nil
}

func (registry *Registry) selectedProvider(capabilityKey string) *entry {
	var fallback *entry
	keys := make([]string, 0, len(registry.providers))
	for key := range registry.providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		provider := registry.providers[key]
		if !contains(provider.registration.CapabilityKeys, capabilityKey) {
			continue
		}
		if fallback == nil {
			fallback = provider
		}
		if provider.registration.Registered && provider.registration.Enabled {
			return provider
		}
	}
	return fallback
}

func providerView(provider *entry) capability.Provider {
	enablement := capability.EnablementDisabled
	if provider.registration.Enabled {
		enablement = capability.EnablementEnabled
	}
	return capability.Provider{
		Key: provider.registration.Key, Adapter: provider.registration.Adapter, Registered: provider.registration.Registered,
		VerifiedCompatibility: []string{}, CapabilityKeys: provider.registration.CapabilityKeys,
		Configuration: configurationFrom(provider.registration.RequiredConfig), Enablement: enablement,
		Health: provider.health, Mode: provider.registration.Mode, Operations: provider.registration.Operations,
		RequiredConfig: provider.registration.RequiredConfig, LastCheckedAt: provider.checkedAt,
		Links: []capability.Link{{Rel: "health-check", Href: "/api/v1/admin/providers/" + provider.registration.Key + "/health-check"}},
	}
}

func coreCapabilities() []capability.Capability {
	definitions := []struct {
		key        string
		operations []string
		link       string
	}{
		{"identity.external-login", []string{"authorize", "callback", "link", "unlink"}, "/api/v1/session/credentials"},
		{"identity.jwks", []string{"get"}, "/oauth2/jwks.json"},
		{"identity.oidc", []string{"authorize", "end_session", "revoke", "token", "userinfo"}, "/.well-known/openid-configuration"},
		{"identity.pat", []string{"create", "list", "revoke", "verify"}, "/api/v1/pat"},
		{"identity.profile", []string{"get", "update", "upload_avatar", "upload_cover"}, "/api/v1/session/me"},
		{"identity.reset-password", []string{"request", "reset"}, "/api/v1/auth/password/forgot"},
		{"identity.user-admin", []string{"audit", "create", "delete", "get", "list", "reset_password", "roles", "update_status"}, "/api/v1/admin/users"},
		{"identity.verify-email", []string{"request", "verify"}, "/api/v1/auth/email/verify-request"},
	}
	items := make([]capability.Capability, 0, len(definitions))
	for _, definition := range definitions {
		items = append(items, capability.Capability{
			Key: definition.key, ContractVersion: "1.0", Support: capability.SupportSupported,
			Configuration: capability.ConfigurationMissing, Enablement: capability.EnablementDisabled,
			Health: capability.HealthUnknown, Operations: definition.operations,
			Links: []capability.Link{{Rel: "manage", Href: definition.link}},
		})
	}
	return items
}

func configurationFrom(fields []capability.ConfigField) capability.Configuration {
	if len(fields) == 0 {
		return capability.ConfigurationComplete
	}
	present := 0
	for _, field := range fields {
		if field.State == capability.ConfigStatePresent {
			present++
		}
	}
	if present == 0 {
		return capability.ConfigurationMissing
	}
	if present == len(fields) {
		return capability.ConfigurationComplete
	}
	return capability.ConfigurationPartial
}

func cloneRegistration(input Registration) Registration {
	output := input
	output.CapabilityKeys = append([]string(nil), input.CapabilityKeys...)
	output.Operations = append([]string(nil), input.Operations...)
	output.RequiredConfig = make([]capability.ConfigField, len(input.RequiredConfig))
	for index, field := range input.RequiredConfig {
		output.RequiredConfig[index] = field
		if field.RotatedAt != nil {
			value := *field.RotatedAt
			output.RequiredConfig[index].RotatedAt = &value
		}
	}
	return output
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func Field(key, value string, secret bool) capability.ConfigField {
	state := capability.ConfigStateMissing
	if strings.TrimSpace(value) != "" {
		state = capability.ConfigStatePresent
	}
	return capability.ConfigField{Key: key, State: state, Secret: secret}
}

func ServiceMetadata() capability.ServiceMetadata {
	return capability.ServiceMetadata{
		Name: "identity", Version: envOr([]string{"PLATFORM_SERVICE_VERSION", "OTEL_SERVICE_VERSION"}, "dev"),
		BuildSHA:   envOr([]string{"PLATFORM_BUILD_SHA", "GITHUB_SHA"}, "unknown"),
		Deployment: envOr([]string{"PLATFORM_DEPLOYMENT_IDENTITY", "HOSTNAME"}, "identity-api"),
	}
}

func envOr(keys []string, fallback string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}
