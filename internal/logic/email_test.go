package logic_test

import (
	"testing"

	"github.com/yueli-official/identity/internal/logic"
)

func TestCanonicalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  A@B.COM ":       "a@b.com",
		"User@Example.com": "user@example.com",
	}
	for in, want := range cases {
		if got := logic.CanonicalizeEmail(in); got != want {
			t.Errorf("Canonicalize(%q)=%q want %q", in, got, want)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	if err := logic.ValidateEmail("a@b.com"); err != nil {
		t.Errorf("valid email rejected: %v", err)
	}
	if err := logic.ValidateEmail("nope"); err == nil {
		t.Error("invalid email accepted")
	}
}
