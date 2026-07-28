package main

import "testing"

func TestParseSeedRejectsUnsafeRedirect(t *testing.T) {
	_, err := parseSeed(`{
		"account":{"id":"user-1","email":"test@example.com","password":"long-enough-password","displayName":"测试用户"},
		"siteClients":[{"id":"shop","redirectUris":["javascript:alert(1)"]}]
	}`)
	if err == nil {
		t.Fatal("expected unsafe redirect to be rejected")
	}
}

func TestParseSeedAcceptsLANAndLoopbackRedirects(t *testing.T) {
	_, err := parseSeed(`{
		"account":{"id":"user-1","email":"test@example.com","password":"long-enough-password","displayName":"测试用户"},
		"siteClients":[{
			"id":"shop-main-web",
			"redirectUris":["http://localhost:3004/auth/callback","http://192.168.5.7:3004/auth/callback"],
			"postLogoutRedirectUris":["http://192.168.5.7:3004/"],
			"audiences":["shop-api"]
		}],
		"serviceClients":[{
			"id":"commerce-asset-svc",
			"secret":"development-secret-at-least-24-characters",
			"secretRef":"COMMERCE_ASSET_SECRET",
			"audience":"asset-api",
			"scopes":["asset:sign"]
		}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
}
