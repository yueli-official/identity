package oidc

import (
	"context"
	"net/url"
	"testing"
)

func TestRedirectSecureCheckerAllowsPrivateHTTPOnlyWhenExplicit(t *testing.T) {
	parse := func(value string) *url.URL {
		parsed, err := url.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}

	strict := redirectSecureChecker(false)
	localDevice := redirectSecureChecker(true)
	ctx := context.Background()

	if !strict(ctx, parse("https://gallery.example.com/auth/callback")) {
		t.Fatal("HTTPS redirect must remain allowed")
	}
	if !strict(ctx, parse("http://gallery.localhost/auth/callback")) {
		t.Fatal("Fosite localhost development redirect must remain allowed")
	}
	if strict(ctx, parse("http://192.168.5.7:3007/auth/callback")) {
		t.Fatal("private HTTP redirect must be denied by default")
	}
	if !localDevice(ctx, parse("http://192.168.5.7:3007/auth/callback")) {
		t.Fatal("explicit local-device mode must allow RFC1918 HTTP redirect")
	}
	if localDevice(ctx, parse("http://203.0.113.10/auth/callback")) {
		t.Fatal("explicit local-device mode must still deny public HTTP redirect")
	}
	if localDevice(ctx, parse("http://gallery.example.com/auth/callback")) {
		t.Fatal("explicit local-device mode must deny non-IP HTTP redirect")
	}
}
