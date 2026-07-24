package authentication

import (
	"bytes"
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestCeremonyStorageOmitsChallengeAndRestoresOnlyMatchingValue(t *testing.T) {
	session := &webauthn.SessionData{
		Challenge: "browser-visible-secret-challenge",
		UserID:    []byte("user-handle"),
	}
	material, options, err := encodeCeremony(
		session,
		map[string]any{"publicKey": map[string]any{"challenge": session.Challenge}},
	)
	if err != nil {
		t.Fatalf("encodeCeremony() error = %v", err)
	}
	if bytes.Contains(material.LibraryState, []byte(session.Challenge)) {
		t.Fatalf("library state persisted the plaintext challenge: %s", material.LibraryState)
	}
	if !bytes.Contains(options.JSON, []byte(session.Challenge)) {
		t.Fatalf("browser options omitted the challenge: %s", options.JSON)
	}

	restored, err := decodeCeremony(material, session.Challenge)
	if err != nil {
		t.Fatalf("decodeCeremony() error = %v", err)
	}
	if restored.Challenge != session.Challenge ||
		string(restored.UserID) != string(session.UserID) {
		t.Fatalf("restored session = %+v", restored)
	}
	if _, err := decodeCeremony(material, "different-challenge"); !errors.Is(err, ErrCeremonyInvalid) {
		t.Fatalf("mismatched challenge error = %v, want ErrCeremonyInvalid", err)
	}
}

func TestWebAuthnVerifierRequiresExplicitRelyingPartyBoundary(t *testing.T) {
	tests := []WebAuthnConfig{
		{RPDisplayName: "Account", RPOrigins: []string{"https://account.example.test"}},
		{RPID: "example.test", RPOrigins: []string{"https://account.example.test"}},
		{RPID: "example.test", RPDisplayName: "Account"},
	}
	for _, config := range tests {
		if _, err := NewWebAuthnVerifier(config); err == nil {
			t.Fatalf("NewWebAuthnVerifier(%+v) succeeded, want configuration error", config)
		}
	}
}
