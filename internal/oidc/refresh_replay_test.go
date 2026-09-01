package oidc

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestRefreshReplayReceiptIsBoundEncryptedAndExpires(t *testing.T) {
	codec, err := NewRefreshReplayCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	key := codec.Digest("site-web", "refresh-secret")
	body := []byte(`{"access_token":"access","refresh_token":"rotated"}`)
	ciphertext, err := codec.Seal(key, body)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("rotated")) {
		t.Fatal("receipt stored token response in plaintext")
	}
	opened, err := codec.Open(key, ciphertext)
	if err != nil || !bytes.Equal(opened, body) {
		t.Fatalf("opened/error = %s / %v", opened, err)
	}
	if _, err := codec.Open(codec.Digest("other-web", "refresh-secret"), ciphertext); err == nil {
		t.Fatal("receipt opened for another client")
	}
	backend := newMemBackend()
	now := time.Unix(100, 0).UTC()
	backend.refresh["active"] = RefreshRecord{RequestID: "family-1", Active: true, ExpiresAt: now.Add(time.Hour)}
	receipt := RefreshReplayReceipt{KeyDigest: key, ClientID: "site-web", RequestID: "family-1", ResponseCiphertext: ciphertext, ExpiresAt: now.Add(time.Second)}
	if err := backend.PutRefreshReplay(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	if _, found, err := backend.GetRefreshReplay(context.Background(), key, "site-web", now); err != nil || !found {
		t.Fatalf("fresh receipt found/error = %v/%v", found, err)
	}
	if _, found, err := backend.GetRefreshReplay(context.Background(), key, "site-web", now.Add(time.Second)); err != nil || found {
		t.Fatalf("expired receipt found/error = %v/%v", found, err)
	}
}
