package identitycap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yueli-official/foundation/go/capability"
)

func TestSnapshotSelectsRuntimeProvidersWithoutExposingSecrets(t *testing.T) {
	registry, err := New(
		Registration{Key: "identity-core", Adapter: "builtin", Registered: true, Enabled: true, CapabilityKeys: []string{"identity.jwks", "identity.oidc", "identity.pat", "identity.profile", "identity.user-admin"}, Operations: []string{"issue"}, RequiredConfig: []capability.ConfigField{Field("global_secret", "core-secret", true)}, Checker: HealthCheckFunc(func(context.Context) error { return nil }), InitialHealth: capability.HealthHealthy},
		Registration{Key: "dev-mail", Adapter: "dev", Registered: true, Enabled: true, CapabilityKeys: []string{"identity.reset-password", "identity.verify-email"}, Operations: []string{"send"}, Checker: HealthCheckFunc(func(context.Context) error { return nil }), InitialHealth: capability.HealthHealthy},
		Registration{Key: "primary-smtp", Adapter: "smtp", Registered: true, CapabilityKeys: []string{"identity.reset-password", "identity.verify-email"}, Operations: []string{"send"}, RequiredConfig: []capability.ConfigField{Field("host", "", false), Field("password", "smtp-secret", true)}},
		Registration{Key: "google", Adapter: "google-oauth", CapabilityKeys: []string{"identity.external-login"}, Operations: []string{"authorize"}, RequiredConfig: []capability.ConfigField{Field("client_id", "", false), Field("client_secret", "google-secret", true), Field("redirect_url", "https://account.test/callback", false)}},
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := registry.Snapshot(testMetadata(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	verify, ok := snapshot.Capability("identity.verify-email")
	if !ok || verify.ProviderInstance != "dev-mail" || verify.Health != capability.HealthHealthy || !verify.Effective {
		t.Fatalf("verify-email capability = %+v, %t", verify, ok)
	}
	external, ok := snapshot.Capability("identity.external-login")
	if !ok || external.ProviderInstance != "" || external.Enablement != capability.EnablementDisabled || external.Effective {
		t.Fatalf("external-login capability = %+v, %t", external, ok)
	}
	google, ok := snapshot.Provider("google")
	if !ok || google.Registered || google.Configuration != capability.ConfigurationPartial || google.Effective {
		t.Fatalf("google provider = %+v, %t", google, ok)
	}
	data, _ := json.Marshal(snapshot.Manifest())
	if strings.Contains(string(data), "core-secret") || strings.Contains(string(data), "smtp-secret") || strings.Contains(string(data), "google-secret") {
		t.Fatalf("manifest leaked a secret: %s", data)
	}
}

func TestHealthProbeRebuildsWithoutMutatingOldSnapshot(t *testing.T) {
	registry, err := New(Registration{
		Key: "dev-mail", Adapter: "dev", Registered: true, Enabled: true,
		CapabilityKeys: []string{"identity.reset-password", "identity.verify-email"}, Operations: []string{"send"},
		Checker: HealthCheckFunc(func(context.Context) error { return errors.New("offline") }),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := registry.Snapshot(testMetadata(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.CheckHealth(context.Background(), "dev-mail"); err == nil {
		t.Fatal("expected probe error")
	}
	second, err := registry.Snapshot(testMetadata(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	before, _ := first.Provider("dev-mail")
	after, _ := second.Provider("dev-mail")
	if first == second || before.Health != capability.HealthUnknown || after.Health != capability.HealthUnhealthy || after.LastCheckedAt == nil {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
}

func TestNewRejectsMultipleEnabledProvidersForOneCapability(t *testing.T) {
	_, err := New(
		Registration{Key: "a", Adapter: "dev", Registered: true, Enabled: true, CapabilityKeys: []string{"identity.verify-email"}},
		Registration{Key: "b", Adapter: "smtp", Registered: true, Enabled: true, CapabilityKeys: []string{"identity.verify-email"}},
	)
	if err == nil {
		t.Fatal("expected duplicate active provider error")
	}
}

func TestNewDeepCopiesRegistrationInput(t *testing.T) {
	rotatedAt := time.Now().UTC()
	expectedRotatedAt := rotatedAt
	registration := Registration{
		Key: "dev-mail", Adapter: "dev", Registered: true, Enabled: true,
		CapabilityKeys: []string{"identity.verify-email"}, Operations: []string{"send"},
		RequiredConfig: []capability.ConfigField{{Key: "token", State: capability.ConfigStatePresent, Secret: true, RotatedAt: &rotatedAt}},
		Checker:        HealthCheckFunc(func(context.Context) error { return nil }), InitialHealth: capability.HealthHealthy,
	}
	registry, err := New(registration)
	if err != nil {
		t.Fatal(err)
	}
	registration.CapabilityKeys[0] = "identity.oidc"
	registration.Operations[0] = "mutated"
	registration.RequiredConfig[0].Key = "mutated"
	*registration.RequiredConfig[0].RotatedAt = rotatedAt.Add(time.Hour)
	snapshot, err := registry.Snapshot(testMetadata(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := snapshot.Provider("dev-mail")
	if provider.CapabilityKeys[0] != "identity.verify-email" || provider.Operations[0] != "send" || provider.RequiredConfig[0].Key != "token" || !provider.RequiredConfig[0].RotatedAt.Equal(expectedRotatedAt) {
		t.Fatalf("registry input mutated through caller slices: %+v", provider)
	}
}

func testMetadata() capability.ServiceMetadata {
	return capability.ServiceMetadata{Name: "identity", Version: "test", BuildSHA: "test", Deployment: "identity-test"}
}
