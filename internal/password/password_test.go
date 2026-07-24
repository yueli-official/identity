package password_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"platform/services/identity/internal/password"
)

func TestPolicyUsesNFCUnicodeCodePointsAndContext(t *testing.T) {
	manager := password.New(password.DefaultConfig())
	decomposed := "Cafe\u0301 horse battery"
	normalized, err := manager.Validate(context.Background(), decomposed, password.Context{})
	if err != nil {
		t.Fatal(err)
	}
	if normalized != "Café horse battery" {
		t.Fatalf("Normalize() = %q", normalized)
	}
	if _, err := manager.Validate(
		context.Background(), "owner@example.test",
		password.Context{Email: "owner@example.test"},
	); password.ParseReason(err) != password.ReasonContext {
		t.Fatalf("context password error = %v", err)
	}
	if _, err := manager.Validate(
		context.Background(), strings.Repeat("界", 129), password.Context{},
	); password.ParseReason(err) != password.ReasonTooLong {
		t.Fatalf("long Unicode password error = %v", err)
	}
}

func TestArgon2idRoundTripAndMalformedHashBounds(t *testing.T) {
	manager := password.New(password.DefaultConfig())
	encoded, err := manager.Hash("Café horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("Hash() = %q", encoded)
	}
	if !manager.Verify(encoded, "Cafe\u0301 horse battery") {
		t.Fatal("canonically equivalent password did not verify")
	}
	if manager.Verify(encoded, "wrong password value") {
		t.Fatal("wrong password verified")
	}
	oversized := "$argon2id$v=19$m=999999999,t=2,p=1$c2FsdHNhbHQ$MTIzNDU2Nzg5MDEyMzQ1Ng"
	if manager.Verify(oversized, "anything") {
		t.Fatal("oversized parameters verified")
	}
}

func TestLegacyBcryptVerifiesAndRequiresRehash(t *testing.T) {
	manager := password.New(password.DefaultConfig())
	legacy, err := bcrypt.GenerateFromPassword([]byte("legacy password value"), 4)
	if err != nil {
		t.Fatal(err)
	}
	if !manager.Verify(string(legacy), "legacy password value") {
		t.Fatal("legacy bcrypt did not verify")
	}
	if !manager.NeedsRehash(string(legacy)) {
		t.Fatal("legacy bcrypt should require rehash")
	}
}

func TestPolicyRejectsBlockedPassword(t *testing.T) {
	config := password.DefaultConfig()
	config.MinLength = 8
	manager := password.New(config)
	_, err := manager.Validate(context.Background(), "password", password.Context{})
	var policyError *password.PolicyError
	if !errors.As(err, &policyError) || policyError.Reason != password.ReasonBlocklist {
		t.Fatalf("blocked password error = %v", err)
	}
}
