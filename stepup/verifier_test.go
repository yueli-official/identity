package stepup

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type staticKey struct {
	key any
}

func (source staticKey) PublicKey(context.Context, string) (any, error) {
	return source.key, nil
}

func TestVerifierBindsActionResourceAndConsumesJTI(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	resourceDigest := sha256.Sum256([]byte("order:123"))
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("step-up+jwt").WithHeader("kid", "test"),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer: "https://identity.example.test", Subject: "user-1",
		Audience: jwt.Audience{"commerce-api"}, ID: "proof-1",
		IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now),
		Expiry: jwt.NewNumericDate(now.Add(2 * time.Minute)),
	}).Claims(map[string]any{
		"token_use": "step_up", "sid": "session-1",
		"action":        "order.refund",
		"resource_hash": base64.RawURLEncoding.EncodeToString(resourceDigest[:]),
		"auth_time":     now.Unix(), "amr": []string{"pwd", "otp"},
		"acr": "urn:yueli:assurance:multi-factor", "recovery": false,
	}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	replay := NewMemoryReplayStore()
	verifier, err := New(Config{
		Keys:   staticKey{key: &privateKey.PublicKey},
		Issuer: "https://identity.example.test", Audience: "commerce-api",
		Replay: replay, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := verifier.VerifyAndConsume(
		context.Background(), raw, "order.refund", "order:123",
	)
	if err != nil || evidence.Subject != "user-1" || len(evidence.Methods) != 2 {
		t.Fatalf("VerifyAndConsume() = %+v, %v", evidence, err)
	}
	if _, err := verifier.VerifyAndConsume(
		context.Background(), raw, "order.refund", "order:123",
	); !errors.Is(err, ErrReplay) {
		t.Fatalf("replay error = %v", err)
	}

	secondReplay := NewMemoryReplayStore()
	wrongResourceVerifier, _ := New(Config{
		Keys:   staticKey{key: &privateKey.PublicKey},
		Issuer: "https://identity.example.test", Audience: "commerce-api",
		Replay: secondReplay, Clock: func() time.Time { return now },
	})
	if _, err := wrongResourceVerifier.VerifyAndConsume(
		context.Background(), raw, "order.refund", "order:other",
	); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("resource mismatch error = %v", err)
	}
}
