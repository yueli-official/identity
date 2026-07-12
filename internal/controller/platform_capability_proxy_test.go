package controller

import (
	"net/http"
	"net/url"
	"testing"
)

func TestCapabilityProxyRouteAllowlist(t *testing.T) {
	tests := []struct {
		method, path, scope string
		ok                  bool
	}{
		{"GET", "/api/v1/admin/platform-proxy/asset/capabilities", "platform:capabilities:read", true},
		{"GET", "/api/v1/admin/platform-proxy/commerce/providers/paypal", "platform:capabilities:read", true},
		{"POST", "/api/v1/admin/platform-proxy/notification/providers/primary-smtp/health-check", "platform:capabilities:probe", true},
		{"POST", "/api/v1/admin/platform-proxy/asset/capabilities", "", false},
		{"DELETE", "/api/v1/admin/platform-proxy/asset/providers/local", "", false},
		{"GET", "/api/v1/admin/platform-proxy/asset/storage-backends", "", false},
		{"GET", "/api/v1/admin/platform-proxy/asset/providers/..", "", false},
		{"GET", "/api/v1/admin/platform-proxy/asset/providers/.", "", false},
		{"GET", "/api/v1/admin/platform-proxy/asset/providers/%2e%2e", "", false},
		{"GET", "/api/v1/admin/platform-proxy/asset/providers/a%2Fb", "", false},
	}
	for _, test := range tests {
		_, _, scope, ok := capabilityProxyRoute(test.method, test.path)
		if ok != test.ok || scope != test.scope {
			t.Fatalf("%s %s => ok=%t scope=%q", test.method, test.path, ok, scope)
		}
	}
}

func TestValidateCapabilityProxyOrigin(t *testing.T) {
	for _, raw := range []string{"file:///tmp/asset", "https://user:pass@example.com", "https://example.com/path", "https://example.com?target=other", "https://example.com#fragment"} {
		if err := validateCapabilityProxyOrigin(raw); err == nil {
			t.Fatalf("validateCapabilityProxyOrigin(%q) accepted unsafe target", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:8082", "https://asset.example.com"} {
		if err := validateCapabilityProxyOrigin(raw); err != nil {
			t.Fatalf("validateCapabilityProxyOrigin(%q): %v", raw, err)
		}
	}
}

func TestNewPlatformCapabilityProxyFailsClosedOnInvalidTarget(t *testing.T) {
	if _, err := NewPlatformCapabilityProxy(nil, nil, "https://identity.example.com", map[string]CapabilityProxyTarget{
		"asset": {BaseURL: "https://asset.example.com/path", Audience: "asset-api"},
	}); err == nil {
		t.Fatal("proxy accepted a target with a path")
	}
	proxy, err := NewPlatformCapabilityProxy(nil, nil, "https://identity.example.com", map[string]CapabilityProxyTarget{
		"asset": {BaseURL: "https://asset.example.com", Audience: "asset-api"},
	})
	if err != nil || proxy.targets["asset"].Audience != "asset-api" {
		t.Fatalf("valid proxy target: proxy=%+v err=%v", proxy, err)
	}
}

func TestCapabilityProxyRedirectPolicyRejectsCrossOrigin(t *testing.T) {
	origin, _ := url.Parse("https://asset.example.com")
	policy := sameOriginRedirectPolicy(origin)
	if err := policy(&http.Request{URL: mustURL(t, "https://asset.example.com/readyz")}, nil); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
	if err := policy(&http.Request{URL: mustURL(t, "https://evil.example/steal")}, nil); err == nil {
		t.Fatal("cross-origin redirect accepted")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
