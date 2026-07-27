package oauthlogin_test

import (
	"testing"

	"github.com/yueli-official/identity/internal/oauthlogin"
)

func TestState_RoundTrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/oauth2/authorize?x=1", "nonce123", "", 9999999999)
	rt, nonce, bind, err := oauthlogin.DecodeState(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if rt != "/oauth2/authorize?x=1" || nonce != "nonce123" || bind != "" {
		t.Fatalf("got %q %q %q", rt, nonce, bind)
	}
}

func TestState_RoundTripWithBind(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/", "n", "identity-123", 9999999999)
	_, _, bind, err := oauthlogin.DecodeState(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if bind != "identity-123" {
		t.Fatalf("bind = %q, want identity-123", bind)
	}
}

func TestState_Tampered(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/x", "n", "", 9999999999)
	if _, _, _, err := oauthlogin.DecodeState(secret, tok+"x"); err == nil {
		t.Fatal("want sig error")
	}
}

func TestState_Expired(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := oauthlogin.EncodeState(secret, "/x", "n", "", 1) // exp in the past
	if _, _, _, err := oauthlogin.DecodeState(secret, tok); err == nil {
		t.Fatal("want expiry error")
	}
}
