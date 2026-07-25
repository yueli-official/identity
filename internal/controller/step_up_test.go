package controller

import (
	"testing"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/authentication"
)

func TestStepUpRequirementValidation(t *testing.T) {
	t.Parallel()

	requirement, ok := stepUpRequirement(v1.StepUpRequirement{
		MinimumLevel:       "aal2",
		MinimumProfile:     "urn:yueli:assurance:multi-factor",
		MinimumFactorCount: 2,
	})
	if !ok {
		t.Fatal("expected the Account admin requirement to be valid")
	}
	if requirement.MinimumLevel != authentication.LevelAAL2 ||
		requirement.MinimumProfile != authentication.ProfileMultiFactor {
		t.Fatalf("unexpected requirement: %#v", requirement)
	}

	for _, test := range []v1.StepUpRequirement{
		{MinimumLevel: "aal4"},
		{MinimumProfile: "urn:yueli:assurance:unknown"},
	} {
		if _, valid := stepUpRequirement(test); valid {
			t.Fatalf("expected invalid requirement: %#v", test)
		}
	}
}
