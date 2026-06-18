package iderr_test

import (
	"testing"

	"platform/gokit/errs"
	"platform/services/identity/internal/iderr"
)

func TestCodesRegisteredWithStatus(t *testing.T) {
	cases := map[string]int{
		iderr.CodeEmailTaken:         409,
		iderr.CodeInvalidCredentials: 401,
		iderr.CodeAccountDisabled:    403,
		iderr.CodeAccountLocked:      429,
		iderr.CodeWeakPassword:       400,
		iderr.CodeInvalidEmail:       400,
		iderr.CodeNotAuthenticated:   401,
		iderr.CodeOAuthEmailConflict: 409,
		iderr.CodeOAuthNoEmail:       400,
		iderr.CodeOAuthFailed:        401,
		iderr.CodeVerificationInvalid: 400,
		iderr.CodeResetThrottled:      429,
		iderr.CodeVerifyThrottled:     429,
	}
	for code, want := range cases {
		if got := errs.Status(code); got != want {
			t.Errorf("Status(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestConstructorsCarryCode(t *testing.T) {
	e := iderr.EmailTaken("a@b.com")
	if e.Code != iderr.CodeEmailTaken {
		t.Fatalf("code = %q", e.Code)
	}
	if e.Params["email"] != "a@b.com" {
		t.Fatalf("params = %#v", e.Params)
	}
	ie := iderr.InvalidEmail("bad")
	if ie.Code != iderr.CodeInvalidEmail {
		t.Fatalf("invalid email code = %q", ie.Code)
	}
}
