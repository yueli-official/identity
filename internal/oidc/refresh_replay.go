package oidc

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

type RefreshReplayReceipt struct {
	KeyDigest          string
	ClientID           string
	RequestID          string
	ResponseCiphertext []byte
	ExpiresAt          time.Time
}

type RefreshReplayStore interface {
	PutRefreshReplay(context.Context, RefreshReplayReceipt) error
	GetRefreshReplay(context.Context, string, string, time.Time) (RefreshReplayReceipt, bool, error)
}

type RefreshReplayCodec struct {
	digestKey []byte
	aead      cipher.AEAD
}

func NewRefreshReplayCodec(secret []byte) (*RefreshReplayCodec, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("OIDC refresh replay secret must contain at least 32 bytes")
	}
	digestKey := hmacKey(secret, "identity/oidc-refresh-replay/digest/v1")
	encryptionKey := hmacKey(secret, "identity/oidc-refresh-replay/encryption/v1")
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &RefreshReplayCodec{digestKey: digestKey, aead: aead}, nil
}
func hmacKey(secret []byte, domain string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(domain))
	return mac.Sum(nil)
}
func (codec *RefreshReplayCodec) Digest(clientID, refresh string) string {
	mac := hmac.New(sha256.New, codec.digestKey)
	_, _ = mac.Write([]byte(clientID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(refresh))
	return hex.EncodeToString(mac.Sum(nil))
}
func (codec *RefreshReplayCodec) Seal(key string, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return codec.aead.Seal(nonce, nonce, plaintext, []byte(key)), nil
}
func (codec *RefreshReplayCodec) Open(key string, ciphertext []byte) ([]byte, error) {
	size := codec.aead.NonceSize()
	if len(ciphertext) < size {
		return nil, fmt.Errorf("OIDC refresh replay receipt is truncated")
	}
	return codec.aead.Open(nil, ciphertext[:size], ciphertext[size:], []byte(key))
}
func (s *Store) PutRefreshReplay(ctx context.Context, receipt RefreshReplayReceipt) error {
	return s.be.PutRefreshReplay(ctx, receipt)
}
func (s *Store) GetRefreshReplay(ctx context.Context, key, client string, now time.Time) (RefreshReplayReceipt, bool, error) {
	return s.be.GetRefreshReplay(ctx, key, client, now)
}
