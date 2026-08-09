package user_test

import (
	"testing"

	"github.com/yueli-official/identity/internal/user"
)

func TestPublicKeyDelegatesToCompactURLProfile(t *testing.T) {
	seen := map[user.PublicKey]bool{}
	for range 128 {
		key, err := user.NewPublicKey()
		if err != nil {
			t.Fatalf("NewPublicKey: %v", err)
		}
		if len(key) != 8 {
			t.Fatalf("public key %q has length %d, want 8", key, len(key))
		}
		parsed, err := user.ParsePublicKey(string(key))
		if err != nil || parsed != key {
			t.Fatalf("ParsePublicKey(%q) = %q, %v", key, parsed, err)
		}
		if seen[key] {
			t.Fatalf("public key repeated in small sample: %q", key)
		}
		seen[key] = true
	}
}

func TestParsePublicKeyRejectsStorageIDsAliasesAndWrongShapes(t *testing.T) {
	for _, value := range []string{
		"", "1234567", "123456789", "019b0000-0000-7000-9000-000000000030",
		"@alice", "usr_0000000000000000000000", "0ABCDEFG", "OABCDEFG",
	} {
		if _, err := user.ParsePublicKey(value); err == nil {
			t.Errorf("ParsePublicKey(%q) succeeded, want rejection", value)
		}
	}
}

func TestPairwiseSubjectUsesOpaquePublicProfile(t *testing.T) {
	subject, err := user.NewPairwiseSubject()
	if err != nil {
		t.Fatalf("NewPairwiseSubject: %v", err)
	}
	if len(subject) != 16 {
		t.Fatalf("pairwise subject %q has length %d, want 16", subject, len(subject))
	}
}

func TestNormalizeHandleCanonicalizesAndValidates(t *testing.T) {
	handle, err := user.NormalizeHandle("  Alice_01  ")
	if err != nil {
		t.Fatalf("NormalizeHandle: %v", err)
	}
	if handle != "alice_01" {
		t.Fatalf("handle = %q, want alice_01", handle)
	}

	for _, value := range []string{
		"ab", "alice-01", "_alice", "alice_", "张三", "a..b",
		"this_handle_is_longer_than_thirty_chars",
	} {
		if _, err := user.NormalizeHandle(value); err == nil {
			t.Errorf("NormalizeHandle(%q) succeeded, want rejection", value)
		}
	}
}

func TestNormalizeHandleRejectsReservedRoutes(t *testing.T) {
	for _, value := range []string{"ADMIN", "api", "oauth", "settings", "support"} {
		if _, err := user.NormalizeHandle(value); err == nil {
			t.Errorf("NormalizeHandle(%q) succeeded, want reserved-name rejection", value)
		}
	}
}

func TestNormalizeOptionalHandleKeepsAbsenceDistinct(t *testing.T) {
	handle, err := user.NormalizeOptionalHandle("   ")
	if err != nil {
		t.Fatalf("blank optional handle: %v", err)
	}
	if handle != "" {
		t.Fatalf("blank optional handle = %q, want empty", handle)
	}
}
