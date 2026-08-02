package user_test

import (
	"strings"
	"testing"

	"github.com/yueli-official/identity/internal/user"
)

func TestNewPublicKeyUsesStableOpaqueShape(t *testing.T) {
	first, err := user.NewPublicKey()
	if err != nil {
		t.Fatalf("NewPublicKey: %v", err)
	}
	second, err := user.NewPublicKey()
	if err != nil {
		t.Fatalf("NewPublicKey second call: %v", err)
	}
	if first == second {
		t.Fatal("independent public keys must not repeat")
	}
	for _, key := range []user.PublicKey{first, second} {
		if len(key) != 26 || !strings.HasPrefix(string(key), "usr_") {
			t.Fatalf("public key %q does not match usr_ + 22 base64url chars", key)
		}
		if _, err := user.ParsePublicKey(string(key)); err != nil {
			t.Fatalf("generated key must parse: %v", err)
		}
	}
}

func TestNewPairwiseSubjectUsesIndependentOpaqueNamespace(t *testing.T) {
	subject, err := user.NewPairwiseSubject()
	if err != nil {
		t.Fatalf("NewPairwiseSubject: %v", err)
	}
	if len(subject) != 26 || !strings.HasPrefix(subject, "psu_") {
		t.Fatalf("pairwise subject = %q, want psu_ + 22 base64url chars", subject)
	}
}

func TestParsePublicKeyRejectsStorageAndHumanIdentifiers(t *testing.T) {
	for _, value := range []string{
		"", "usr_short", "019b0000-0000-7000-9000-000000000030",
		"@alice", "usr_000000000000000000000!",
	} {
		if _, err := user.ParsePublicKey(value); err == nil {
			t.Errorf("ParsePublicKey(%q) succeeded, want rejection", value)
		}
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
