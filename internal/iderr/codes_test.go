package iderr_test

import (
	"testing"
	"time"

	"platform/services/identity/internal/iderr"
)

func TestCodesDeclareExpectedStatus(t *testing.T) {
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
		descriptor, ok := iderr.DescriptorForCode(code)
		if !ok {
			t.Errorf("DescriptorForCode(%q) is missing", code)
			continue
		}
		if got := descriptor.Kind().Status(); got != want {
			t.Errorf("Status(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestConstructorsCarryCode(t *testing.T) {
	e, ok := iderr.Resolve(iderr.EmailTaken("a@b.com"))
	if !ok {
		t.Fatal("email error did not resolve")
	}
	if e.Code != iderr.CodeEmailTaken {
		t.Fatalf("code = %q", e.Code)
	}
	if e.Params["email"] != "a@b.com" {
		t.Fatalf("params = %#v", e.Params)
	}
	ie, ok := iderr.Resolve(iderr.InvalidEmail("bad"))
	if !ok {
		t.Fatal("invalid email error did not resolve")
	}
	if ie.Code != iderr.CodeInvalidEmail {
		t.Fatalf("invalid email code = %q", ie.Code)
	}

	profile, _ := iderr.Resolve(iderr.InvalidProfile(iderr.ProfileReasonUnsupportedImage))
	if profile.Params["reason"] != "unsupported_image" {
		t.Fatalf("profile params = %#v", profile.Params)
	}

	retryAt := time.Date(2026, 7, 26, 12, 34, 56, 123, time.UTC)
	locked, _ := iderr.Resolve(iderr.AccountLockedUntil(retryAt))
	if locked.Params["retryAt"] != retryAt.Format(time.RFC3339Nano) {
		t.Fatalf("locked params = %#v", locked.Params)
	}

	publisher, _ := iderr.Resolve(iderr.PublisherAttestationInvalid(iderr.PublisherAttestationReasonCommand))
	if publisher.Params["reason"] != "command_invalid" {
		t.Fatalf("publisher params = %#v", publisher.Params)
	}

	scope, _ := iderr.Resolve(iderr.PATScopeInvalid(3))
	if scope.Params["reason"] != "invalid_scope" || scope.Params["index"] != 3 {
		t.Fatalf("scope params = %#v", scope.Params)
	}
	scopeCount, _ := iderr.Resolve(iderr.PATScopesTooMany(50))
	if scopeCount.Params["reason"] != "too_many_scopes" || scopeCount.Params["max"] != 50 {
		t.Fatalf("scope count params = %#v", scopeCount.Params)
	}
}
