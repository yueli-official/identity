package authentication

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestSecretBoxRoundTripBindsCiphertextToAuthenticator(t *testing.T) {
	box, err := NewSecretBox([]byte("test-master-secret-at-least-thirty-two-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Seal([]byte("TOTP-SECRET"), []byte("identity-1|totp-1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("TOTP-SECRET")) {
		t.Fatal("ciphertext contains plaintext")
	}
	plaintext, err := box.Open(ciphertext, []byte("identity-1|totp-1"))
	if err != nil || string(plaintext) != "TOTP-SECRET" {
		t.Fatalf("Open() = %q, %v", plaintext, err)
	}
	if _, err := box.Open(ciphertext, []byte("identity-2|totp-1")); err == nil {
		t.Fatal("ciphertext opened with different identity/authenticator binding")
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Open(tampered, []byte("identity-1|totp-1")); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestStandardTOTPValidatesSkewOncePerTimeStep(t *testing.T) {
	adapter, err := NewTOTPVerifier("Yueli Account")
	if err != nil {
		t.Fatal(err)
	}
	seed, err := adapter.Generate("user@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if seed.Secret == "" || !strings.HasPrefix(seed.URI, "otpauth://totp/") ||
		seed.Digits != 6 || seed.Period != 30 {
		t.Fatalf("seed = %+v", seed)
	}
	now := time.Date(2026, 7, 24, 12, 0, 5, 0, time.UTC)
	code, err := totp.GenerateCodeCustom(seed.Secret, now, totp.ValidateOpts{
		Period: 30, Skew: 0, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatal(err)
	}
	step, valid, err := adapter.Verify(seed.Secret, code, now, nil)
	if err != nil || !valid {
		t.Fatalf("Verify() = %d, %t, %v", step, valid, err)
	}
	if replayStep, replayValid, err := adapter.Verify(
		seed.Secret, code, now, &step,
	); err != nil || replayValid || replayStep != 0 {
		t.Fatalf("replay Verify() = %d, %t, %v", replayStep, replayValid, err)
	}
	if _, valid, err := adapter.Verify(seed.Secret, "123", now, nil); err != nil || valid {
		t.Fatalf("short code valid = %t, error = %v", valid, err)
	}
}

func TestRecoveryCodesAreHighEntropyOneTimeLookupMaterial(t *testing.T) {
	codec, err := NewRecoveryCodeCodec([]byte("test-master-secret-at-least-thirty-two-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	codes, digests, err := codec.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != recoveryCodeCount || len(digests) != recoveryCodeCount {
		t.Fatalf("generated %d codes and %d digests", len(codes), len(digests))
	}
	seen := map[string]struct{}{}
	for index, code := range codes {
		if len(code) != 19 || strings.Count(code, "-") != 3 {
			t.Fatalf("code %q has unexpected format", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate recovery code %q", code)
		}
		seen[code] = struct{}{}
		if len(digests[index]) != sha256.Size ||
			!hmac.Equal(digests[index], codec.Digest(strings.ToLower(code))) {
			t.Fatalf("digest %d does not match canonical code", index)
		}
		if bytes.Contains(digests[index], []byte(strings.ReplaceAll(code, "-", ""))) {
			t.Fatalf("digest %d contains plaintext code", index)
		}
	}
}
