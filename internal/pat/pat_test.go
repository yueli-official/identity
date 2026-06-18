package pat_test

import (
	"strings"
	"testing"

	"platform/services/identity/internal/pat"
)

// TestGenerate checks uniqueness, prefix, and length of generated tokens.
func TestGenerate(t *testing.T) {
	t.Run("two calls produce different tokens", func(t *testing.T) {
		a, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		b, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if a == b {
			t.Errorf("Generate() returned identical tokens: %q", a)
		}
	})

	t.Run("token starts with pat_ prefix", func(t *testing.T) {
		tok, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if !strings.HasPrefix(tok, pat.Prefix) {
			t.Errorf("Generate() = %q; want prefix %q", tok, pat.Prefix)
		}
	})

	t.Run("token has length 68", func(t *testing.T) {
		tok, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		// "pat_" (4) + hex(32 bytes) (64) = 68
		if len(tok) != 68 {
			t.Errorf("Generate() len = %d; want 68 (token=%q)", len(tok), tok)
		}
	})
}

// TestHash checks determinism, sensitivity to key/token, and output length.
func TestHash(t *testing.T) {
	key := pat.DeriveKey("test-secret")

	t.Run("same key+token produces same hash", func(t *testing.T) {
		h1 := pat.Hash(key, "pat_abc123")
		h2 := pat.Hash(key, "pat_abc123")
		if h1 != h2 {
			t.Errorf("Hash not deterministic: %q vs %q", h1, h2)
		}
	})

	t.Run("different tokens produce different hashes", func(t *testing.T) {
		h1 := pat.Hash(key, "pat_aaaa")
		h2 := pat.Hash(key, "pat_bbbb")
		if h1 == h2 {
			t.Errorf("Hash collision for different tokens")
		}
	})

	t.Run("different keys produce different hashes for same token", func(t *testing.T) {
		key2 := pat.DeriveKey("other-secret")
		h1 := pat.Hash(key, "pat_same")
		h2 := pat.Hash(key2, "pat_same")
		if h1 == h2 {
			t.Errorf("Hash collision for different keys")
		}
	})

	t.Run("output is 64 hex chars", func(t *testing.T) {
		h := pat.Hash(key, "pat_token")
		if len(h) != 64 {
			t.Errorf("Hash len = %d; want 64 (hash=%q)", len(h), h)
		}
	})
}

// TestParse checks prefix validation and return values.
func TestParse(t *testing.T) {
	t.Run("generated token parses ok", func(t *testing.T) {
		tok, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		got, ok := pat.Parse(tok)
		if !ok {
			t.Errorf("Parse(%q) ok=false; want true", tok)
		}
		if got != tok {
			t.Errorf("Parse(%q) returned %q; want unchanged token", tok, got)
		}
	})

	t.Run("prefix alone returns ok=false", func(t *testing.T) {
		_, ok := pat.Parse("pat_")
		if ok {
			t.Errorf("Parse(%q) ok=true; want false", "pat_")
		}
	})

	t.Run("empty string returns ok=false", func(t *testing.T) {
		_, ok := pat.Parse("")
		if ok {
			t.Errorf("Parse(\"\") ok=true; want false")
		}
	})

	t.Run("string without prefix returns ok=false", func(t *testing.T) {
		_, ok := pat.Parse("nbp_sometoken")
		if ok {
			t.Errorf("Parse(\"nbp_sometoken\") ok=true; want false")
		}
	})
}

// TestDeriveKey checks determinism, key length, and uniqueness across secrets.
func TestDeriveKey(t *testing.T) {
	t.Run("same secret yields identical key", func(t *testing.T) {
		k1 := pat.DeriveKey("my-secret")
		k2 := pat.DeriveKey("my-secret")
		if string(k1) != string(k2) {
			t.Errorf("DeriveKey not deterministic for same secret")
		}
	})

	t.Run("key length is 32", func(t *testing.T) {
		k := pat.DeriveKey("some-secret")
		if len(k) != 32 {
			t.Errorf("DeriveKey len = %d; want 32", len(k))
		}
	})

	t.Run("empty secret yields deterministic non-empty 32-byte key", func(t *testing.T) {
		k1 := pat.DeriveKey("")
		k2 := pat.DeriveKey("")
		if len(k1) != 32 {
			t.Errorf("DeriveKey(\"\") len = %d; want 32", len(k1))
		}
		if string(k1) != string(k2) {
			t.Errorf("DeriveKey(\"\") not deterministic")
		}
		allZero := true
		for _, b := range k1 {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("DeriveKey(\"\") returned all-zero key")
		}
	})

	t.Run("different secrets yield different keys", func(t *testing.T) {
		k1 := pat.DeriveKey("secret-a")
		k2 := pat.DeriveKey("secret-b")
		if string(k1) == string(k2) {
			t.Errorf("DeriveKey collision for different secrets")
		}
	})
}

// TestDisplay checks the display fragment behaviour and the short-token guard.
func TestDisplay(t *testing.T) {
	t.Run("normal token returns first PrefixLen chars", func(t *testing.T) {
		tok, err := pat.Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		d := pat.Display(tok)
		if d != tok[:pat.PrefixLen] {
			t.Errorf("Display(%q) = %q; want %q", tok, d, tok[:pat.PrefixLen])
		}
	})

	t.Run("short token does not panic and returns whole token", func(t *testing.T) {
		short := "pat_"
		d := pat.Display(short)
		if d != short {
			t.Errorf("Display(%q) = %q; want %q", short, d, short)
		}
	})
}
