package user_test

import (
	"strings"
	"testing"

	"github.com/yueli-official/identity/internal/user"
)

func TestNormalizeProfileUpdateTrimsOwnedFields(t *testing.T) {
	got, err := user.NormalizeProfileUpdate(user.ProfileUpdate{
		DisplayName: "  月 离  ",
		Handle:      " Alice_01 ",
		Bio:         "  hello  ",
		Locale:      " zh-CN ",
	})
	if err != nil {
		t.Fatalf("NormalizeProfileUpdate: %v", err)
	}
	if got.DisplayName != "月 离" || got.Handle != "alice_01" || got.Bio != "hello" || got.Locale != "zh-CN" {
		t.Fatalf("unexpected normalized profile: %+v", got)
	}
}

func TestNormalizeProfileUpdateRejectsInvalidBounds(t *testing.T) {
	tests := []user.ProfileUpdate{
		{DisplayName: "   "},
		{DisplayName: strings.Repeat("名", 81)},
		{DisplayName: "Alice", Bio: strings.Repeat("x", 501)},
		{DisplayName: "Alice", Locale: strings.Repeat("x", 36)},
	}
	for _, input := range tests {
		if _, err := user.NormalizeProfileUpdate(input); err == nil {
			t.Errorf("NormalizeProfileUpdate(%+v) succeeded, want rejection", input)
		}
	}
}
