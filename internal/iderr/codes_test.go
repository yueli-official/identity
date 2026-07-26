package iderr_test

import (
	"testing"
	"time"

	"platform/gokit/errs"
	"platform/services/identity/internal/iderr"
)

func TestCodesRegisteredWithStatus(t *testing.T) {
	cases := map[string]int{
		iderr.CodeEmailTaken:               409,
		iderr.CodeInvalidCredentials:       401,
		iderr.CodeAccountDisabled:          403,
		iderr.CodeAccountLocked:            429,
		iderr.CodeWeakPassword:             400,
		iderr.CodeInvalidEmail:             400,
		iderr.CodeNotAuthenticated:         401,
		iderr.CodeOAuthEmailConflict:       409,
		iderr.CodeOAuthNoEmail:             400,
		iderr.CodeOAuthFailed:              401,
		iderr.CodeVerificationInvalid:      400,
		iderr.CodeResetThrottled:           429,
		iderr.CodeVerifyThrottled:          429,
		iderr.CodeSelfAdminActionForbidden: 403,
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

	profile := iderr.InvalidProfile(iderr.ProfileReasonUnsupportedImage)
	if profile.Params["reason"] != "unsupported_image" {
		t.Fatalf("profile params = %#v", profile.Params)
	}

	retryAt := time.Date(2026, 7, 26, 12, 34, 56, 123, time.UTC)
	locked := iderr.AccountLockedUntil(retryAt)
	if locked.Params["retryAt"] != retryAt.Format(time.RFC3339Nano) {
		t.Fatalf("locked params = %#v", locked.Params)
	}

	publisher := iderr.PublisherAttestationInvalid(iderr.PublisherAttestationReasonCommand)
	if publisher.Params["reason"] != "command_invalid" {
		t.Fatalf("publisher params = %#v", publisher.Params)
	}

	scope := iderr.PATScopeInvalid(3)
	if scope.Params["reason"] != "invalid_scope" || scope.Params["index"] != 3 {
		t.Fatalf("scope params = %#v", scope.Params)
	}
	scopeCount := iderr.PATScopesTooMany(50)
	if scopeCount.Params["reason"] != "too_many_scopes" || scopeCount.Params["max"] != 50 {
		t.Fatalf("scope count params = %#v", scopeCount.Params)
	}
}
