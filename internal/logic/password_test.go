package logic_test

import (
	"testing"

	"github.com/yueli-official/identity/internal/logic"
)

func TestHashAndVerifyPassword(t *testing.T) {
	h, err := logic.HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !logic.VerifyPassword(h, "correct horse battery") {
		t.Error("verify failed for correct password")
	}
	if logic.VerifyPassword(h, "wrong") {
		t.Error("verify passed for wrong password")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	if err := logic.ValidatePasswordStrength("short"); err == nil {
		t.Error("short password accepted")
	}
	if err := logic.ValidatePasswordStrength("correct horse battery"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
}
