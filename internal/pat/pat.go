// Package pat provides a pure token codec for Personal Access Tokens (PATs).
// It handles token generation, HMAC hashing, prefix validation, and HKDF-based
// key derivation. It has no database or framework dependencies.
package pat

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// Prefix is the human-readable prefix for all PATs.
// The legacy plugin used "nbp_"; new platform tokens use "pat_".
const Prefix = "pat_"

// PrefixLen is the length of the non-secret display fragment stored in the
// database: the prefix (4 chars) plus the first 8 hex characters of the random
// body, totalling 12 characters.
const PrefixLen = 12

// devFallbackSecret is used by DeriveKey when no secret is provided, so that
// hermetic tests work without any configuration. The caller is responsible for
// logging a warning when the fallback is in use.
const devFallbackSecret = "pat-dev-fallback-key"

// hkdfInfo is the fixed HKDF context/info string for PAT HMAC key derivation.
const hkdfInfo = "pat-token-hmac"

// DeriveKey derives a 32-byte HMAC key from secret via HKDF-SHA256.
// If secret is empty, a fixed development fallback is used so that hermetic
// tests work without configuring a real secret. The caller should log a warning
// in that case. The result is deterministic for a given secret.
func DeriveKey(secret string) []byte {
	src := secret
	if src == "" {
		src = devFallbackSecret
	}
	r := hkdf.New(sha256.New, []byte(src), nil, []byte(hkdfInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		// HKDF over SHA-256 with a 32-byte output never fails in practice;
		// returning a zeroed key here keeps the signature clean while making
		// the failure visible (HMAC output will differ from any valid hash).
		return make([]byte, 32)
	}
	return key
}

// Generate returns a fresh plaintext PAT: Prefix + hex(32 random bytes).
// The total length is 4 + 64 = 68 characters.
// It propagates any error from crypto/rand.
func Generate() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return Prefix + hex.EncodeToString(raw), nil
}

// Hash returns HMAC-SHA256(key, token) as a lowercase hex string (64 chars).
// The result is deterministic for a given key and token pair.
func Hash(key []byte, token string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// Parse validates that presented starts with Prefix and has at least one
// additional character after the prefix. It returns (presented, true) on
// success and ("", false) if the token is malformed.
func Parse(presented string) (string, bool) {
	if !strings.HasPrefix(presented, Prefix) {
		return "", false
	}
	// Require at least one char beyond the prefix itself.
	if len(presented) <= len(Prefix) {
		return "", false
	}
	return presented, true
}

// Display returns the non-secret display fragment of a token: the first
// PrefixLen characters. If the token is shorter than PrefixLen, the entire
// token is returned (guard against panic on short/malformed inputs).
func Display(token string) string {
	if len(token) <= PrefixLen {
		return token
	}
	return token[:PrefixLen]
}
