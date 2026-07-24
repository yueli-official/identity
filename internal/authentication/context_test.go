package authentication

import (
	"slices"
	"testing"
	"time"
)

func TestEvaluateRequiresObservedFactsAndRecency(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	value := Context{
		EventID:           "event-1",
		AuthenticatedAt:   now.Add(-2 * time.Minute),
		Methods:           []Method{MethodWebAuthn},
		FactorClasses:     []FactorClass{FactorPossession, FactorLocalUserVerification},
		Level:             LevelAAL2,
		Profile:           ProfilePhishingResistant,
		UserVerified:      true,
		PhishingResistant: true,
		PolicyVersion:     CurrentPolicyVersion,
	}
	got := Evaluate(value, Requirement{
		FreshWithin: 5 * time.Minute, MinimumLevel: LevelAAL2, MinimumProfile: ProfileMultiFactor,
		UserVerification: true, PhishingResistant: true, MinimumFactorCount: 2,
	}, now)
	if !got.Satisfied || len(got.Missing) != 0 {
		t.Fatalf("decision = %+v", got)
	}
}

func TestEvaluateRejectsRecoveryAndFutureAuthentication(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	value := Context{
		EventID: "event-1", AuthenticatedAt: now.Add(time.Minute),
		Methods: []Method{MethodRecovery}, Level: LevelAAL2, Profile: ProfileMultiFactor,
		Recovery: true, PolicyVersion: CurrentPolicyVersion,
	}
	got := Evaluate(value, Requirement{FreshWithin: 5 * time.Minute}, now)
	if got.Satisfied {
		t.Fatalf("decision unexpectedly satisfied: %+v", got)
	}
	for _, want := range []string{"fresh_authentication", "non_recovery_authentication"} {
		if !slices.Contains(got.Missing, want) {
			t.Fatalf("missing %q in %+v", want, got.Missing)
		}
	}
}

func TestNormalizeLegacyDoesNotInventStrongFacts(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	got := NormalizeLegacy(Context{}, at)
	if got.EventID == "" || !got.AuthenticatedAt.Equal(at) {
		t.Fatalf("context = %+v", got)
	}
	if len(got.Methods) != 1 || got.Methods[0] != MethodLegacy ||
		got.Level != LevelAAL1 || got.Profile != ProfileBaseline || got.UserVerified || got.PhishingResistant {
		t.Fatalf("legacy context invented facts: %+v", got)
	}
}
