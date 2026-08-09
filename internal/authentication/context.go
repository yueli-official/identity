// Package authentication owns transport- and storage-neutral authentication
// facts and assurance evaluation for Identity.
package authentication

import (
	"slices"
	"time"

	"github.com/yueli-official/foundation/go/identifier"
)

type Method string

const (
	MethodLegacy    Method = "legacy"
	MethodPassword  Method = "pwd"
	MethodFederated Method = "federated"
	MethodWebAuthn  Method = "webauthn"
	MethodOTP       Method = "otp"
	MethodRecovery  Method = "recovery"
)

type FactorClass string

const (
	FactorKnowledge             FactorClass = "knowledge"
	FactorPossession            FactorClass = "possession"
	FactorLocalUserVerification FactorClass = "local_user_verification"
	FactorFederatedAssertion    FactorClass = "federated_assertion"
)

type Profile string

const (
	ProfileBaseline          Profile = "urn:yueli:assurance:baseline"
	ProfileMultiFactor       Profile = "urn:yueli:assurance:multi-factor"
	ProfilePhishingResistant Profile = "urn:yueli:assurance:phishing-resistant"
	CurrentPolicyVersion             = 1
)

type Level string

const (
	LevelUnknown Level = "unknown"
	LevelAAL1    Level = "aal1"
	LevelAAL2    Level = "aal2"
	LevelAAL3    Level = "aal3"
)

// Context is an immutable snapshot of the facts observed during one successful
// authentication event. Token issuance time and page state are deliberately
// absent: callers must use AuthenticatedAt for recency and OIDC auth_time.
type Context struct {
	EventID           string        `json:"eventId"`
	AuthenticatedAt   time.Time     `json:"authenticatedAt"`
	Methods           []Method      `json:"methods"`
	FactorClasses     []FactorClass `json:"factorClasses"`
	Level             Level         `json:"level"`
	Profile           Profile       `json:"profile"`
	UserVerified      bool          `json:"userVerified"`
	PhishingResistant bool          `json:"phishingResistant"`
	Recovery          bool          `json:"recovery"`
	CredentialRefs    []string      `json:"credentialRefs,omitempty"`
	PolicyVersion     int           `json:"policyVersion"`
}

// Session is the durable Identity login container. PostgreSQL is authoritative;
// caches must preserve this structure without dropping authentication facts.
type Session struct {
	ID             string
	IdentityID     string
	CreatedAt      time.Time
	LastSeen       time.Time
	UserAgent      string
	IP             string
	Device         string
	ExpiresAt      time.Time
	Authentication Context
}

func Password(eventID string, at time.Time) Context {
	return normalize(Context{
		EventID:         eventID,
		AuthenticatedAt: at,
		Methods:         []Method{MethodPassword},
		FactorClasses:   []FactorClass{FactorKnowledge},
		Level:           LevelAAL1,
		Profile:         ProfileBaseline,
		PolicyVersion:   CurrentPolicyVersion,
	}, at)
}

// Federated is intentionally baseline unless a provider-specific adapter later
// supplies and validates upstream authentication facts.
func Federated(eventID string, at time.Time, credentialRef string) Context {
	return normalize(Context{
		EventID:         eventID,
		AuthenticatedAt: at,
		Methods:         []Method{MethodFederated},
		FactorClasses:   []FactorClass{FactorFederatedAssertion},
		Level:           LevelAAL1,
		Profile:         ProfileBaseline,
		CredentialRefs:  nonEmptyStrings(credentialRef),
		PolicyVersion:   CurrentPolicyVersion,
	}, at)
}

func MultiFactor(primary Context, eventID string, at time.Time, credentialRef string) Context {
	methods := append(slices.Clone(primary.Methods), MethodOTP)
	factors := append(slices.Clone(primary.FactorClasses), FactorPossession)
	return normalize(Context{
		EventID: eventID, AuthenticatedAt: at, Methods: methods,
		FactorClasses: uniqueFactors(factors), Level: LevelAAL2,
		Profile:        ProfileMultiFactor,
		CredentialRefs: append(slices.Clone(primary.CredentialRefs), credentialRef),
		PolicyVersion:  CurrentPolicyVersion,
	}, at)
}

func Recovery(primary Context, eventID string, at time.Time) Context {
	return normalize(Context{
		EventID: eventID, AuthenticatedAt: at,
		Methods:       append(slices.Clone(primary.Methods), MethodRecovery),
		FactorClasses: slices.Clone(primary.FactorClasses),
		Level:         LevelAAL1, Profile: ProfileBaseline, Recovery: true,
		CredentialRefs: slices.Clone(primary.CredentialRefs),
		PolicyVersion:  CurrentPolicyVersion,
	}, at)
}

func Passkey(eventID string, at time.Time, credentialRef string, userVerified bool) Context {
	factors := []FactorClass{FactorPossession}
	level := LevelAAL1
	if userVerified {
		factors = append(factors, FactorLocalUserVerification)
		level = LevelAAL2
	}
	return normalize(Context{
		EventID:           eventID,
		AuthenticatedAt:   at,
		Methods:           []Method{MethodWebAuthn},
		FactorClasses:     factors,
		Level:             level,
		Profile:           ProfilePhishingResistant,
		UserVerified:      userVerified,
		PhishingResistant: true,
		CredentialRefs:    nonEmptyStrings(credentialRef),
		PolicyVersion:     CurrentPolicyVersion,
	}, at)
}

// NormalizeLegacy makes pre-E1/test sessions explicit instead of treating an
// empty context as recent strong authentication.
func NormalizeLegacy(value Context, fallbackAt time.Time) Context {
	return normalize(value, fallbackAt)
}

func normalize(value Context, fallbackAt time.Time) Context {
	if value.EventID == "" {
		value.EventID = identifier.MustNew().String()
	}
	if value.AuthenticatedAt.IsZero() {
		value.AuthenticatedAt = fallbackAt
	}
	if value.AuthenticatedAt.IsZero() {
		value.AuthenticatedAt = time.Now().UTC()
	}
	if len(value.Methods) == 0 {
		value.Methods = []Method{MethodLegacy}
	}
	if value.Profile == "" {
		value.Profile = ProfileBaseline
	}
	if value.Level == "" || value.Level == LevelUnknown {
		value.Level = LevelAAL1
	}
	if value.PolicyVersion <= 0 {
		value.PolicyVersion = CurrentPolicyVersion
	}
	value.Methods = slices.Clone(value.Methods)
	value.FactorClasses = slices.Clone(value.FactorClasses)
	value.CredentialRefs = slices.Clone(value.CredentialRefs)
	return value
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func uniqueFactors(values []FactorClass) []FactorClass {
	seen := make(map[FactorClass]struct{}, len(values))
	out := make([]FactorClass, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; value == "" || exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

type Requirement struct {
	FreshWithin        time.Duration
	MinimumLevel       Level
	MinimumProfile     Profile
	UserVerification   bool
	PhishingResistant  bool
	MinimumFactorCount int
	RecoveryAllowed    bool
}

type Decision struct {
	Satisfied bool
	Missing   []string
}

func Evaluate(value Context, requirement Requirement, now time.Time) Decision {
	missing := make([]string, 0, 5)
	if requirement.FreshWithin > 0 &&
		(value.AuthenticatedAt.IsZero() || now.Sub(value.AuthenticatedAt) < 0 ||
			now.Sub(value.AuthenticatedAt) > requirement.FreshWithin) {
		missing = append(missing, "fresh_authentication")
	}
	if profileRank(value.Profile) < profileRank(requirement.MinimumProfile) {
		missing = append(missing, "assurance_profile")
	}
	if levelRank(value.Level) < levelRank(requirement.MinimumLevel) {
		missing = append(missing, "assurance_level")
	}
	if requirement.UserVerification && !value.UserVerified {
		missing = append(missing, "user_verification")
	}
	if requirement.PhishingResistant && !value.PhishingResistant {
		missing = append(missing, "phishing_resistance")
	}
	if requirement.MinimumFactorCount > len(value.FactorClasses) {
		missing = append(missing, "factor_count")
	}
	if value.Recovery && !requirement.RecoveryAllowed {
		missing = append(missing, "non_recovery_authentication")
	}
	return Decision{Satisfied: len(missing) == 0, Missing: missing}
}

func levelRank(value Level) int {
	switch value {
	case LevelAAL3:
		return 3
	case LevelAAL2:
		return 2
	case LevelAAL1:
		return 1
	default:
		return 0
	}
}

func profileRank(value Profile) int {
	switch value {
	case ProfilePhishingResistant:
		return 3
	case ProfileMultiFactor:
		return 2
	case ProfileBaseline:
		return 1
	default:
		return 0
	}
}

func MethodStrings(methods []Method) []string {
	out := make([]string, len(methods))
	for i, method := range methods {
		out[i] = string(method)
	}
	return out
}

func FactorStrings(factors []FactorClass) []string {
	out := make([]string, len(factors))
	for i, factor := range factors {
		out[i] = string(factor)
	}
	return out
}

func Methods(values []string) []Method {
	out := make([]Method, len(values))
	for i, value := range values {
		out[i] = Method(value)
	}
	return out
}

func Factors(values []string) []FactorClass {
	out := make([]FactorClass, len(values))
	for i, value := range values {
		out[i] = FactorClass(value)
	}
	return out
}
