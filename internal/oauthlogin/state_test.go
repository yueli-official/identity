package oauthlogin_test

import (
	"testing"

	"platform/services/identity/internal/oauthlogin"
)

func TestState_RoundTrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/oauth2/authorize?x=1", "nonce123", 9999999999)
	rt, nonce, err := oauthlogin.DecodeState(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if rt != "/oauth2/authorize?x=1" || nonce != "nonce123" {
		t.Fatalf("got %q %q", rt, nonce)
	}
}

func TestState_Tampered(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/x", "n", 9999999999)
	if _, _, err := oauthlogin.DecodeState(secret, tok+"x"); err == nil {
		t.Fatal("want sig error")
	}
}

func TestState_Expired(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/x", "n", 1) // exp in the past
	if _, _, err := oauthlogin.DecodeState(secret, tok); err == nil {
		t.Fatal("want expiry error")
	}
}
