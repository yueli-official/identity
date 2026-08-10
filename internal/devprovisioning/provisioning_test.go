package devprovisioning

import (
	"strings"
	"testing"
)

func TestParseClientsOnlyAcceptsClientsAndNormalizesNonSecretFields(t *testing.T) {
	declared, err := Parse(`{
		"siteClients":[{"id":" nav-web ","redirectUris":["http://localhost:3006/auth/callback"],"audiences":[" identity-api ","identity-api"]}],
		"serviceClients":[{"id":"nav-worker","secret":"this-is-a-long-local-secret","secretRef":" env:NAV_SECRET ","audience":" nav-api ","scopes":["nav:write","nav:write"]}]
	}`, "TEST_PROVISIONING", ClientsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if declared.Account != nil || declared.SiteClients[0].ID != "nav-web" ||
		declared.SiteClients[0].PostLogoutRedirectURIs == nil ||
		len(declared.SiteClients[0].Audiences) != 1 ||
		declared.ServiceClients[0].SecretRef != "env:NAV_SECRET" ||
		len(declared.ServiceClients[0].Scopes) != 1 {
		t.Fatalf("declaration = %#v", declared)
	}
}

func TestParseClientsOnlyRejectsAccountEmptyAndUnknownFields(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "account", raw: `{"account":{"email":"test@example.test"},"siteClients":[{"id":"nav-web","redirectUris":["http://localhost/callback"]}]}`, want: "must contain only"},
		{name: "empty", raw: `{}`, want: "at least one OIDC client"},
		{name: "unknown", raw: `{"clients":[]}`, want: "unknown field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.raw, "TEST_PROVISIONING", ClientsOnly); err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestParseRejectsDuplicateClientIdentityAndInvalidRedirect(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "duplicate across client kinds",
			raw:  `{"siteClients":[{"id":"shared","redirectUris":["http://localhost/callback"]}],"serviceClients":[{"id":"shared","secret":"this-is-a-long-local-secret","secretRef":"env:SECRET","audience":"api","scopes":["write"]}]}`,
			want: "unique id",
		},
		{
			name: "redirect credentials",
			raw:  `{"siteClients":[{"id":"site","redirectUris":["http://user:pass@localhost/callback"]}]}`,
			want: "invalid redirect URI",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Parse(test.raw, "TEST_PROVISIONING", ClientsOnly); err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestParseFullSeedRequiresAndNormalizesIdentityFixture(t *testing.T) {
	declared, err := Parse(`{
		"account":{"id":"019c52f0-0000-7000-8000-000000000001","userKey":"TestA123","email":"test@example.test","password":"long-enough-password","handle":"Test_Admin","displayName":"Test Admin"}
	}`, "TEST_SEED", FullSeed)
	if err != nil {
		t.Fatal(err)
	}
	if declared.Account == nil || declared.Account.UserKey != "TestA123" || declared.Account.Handle != "test_admin" {
		t.Fatalf("account = %#v", declared.Account)
	}
}
