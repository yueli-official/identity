package identityabuse

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/yueli-official/foundation/go/abuse"
)

const (
	ActionRegister        abuse.ActionKey = "identity.register"
	ActionLogin           abuse.ActionKey = "identity.password_login"
	ActionPasskeyCeremony abuse.ActionKey = "identity.passkey_ceremony"
	ActionMFAVerification abuse.ActionKey = "identity.mfa_verification"
)

type Policy struct {
	LoginAccountCapacity   int64
	LoginNetworkCapacity   int64
	LoginWindow            time.Duration
	RegisterCapacity       int64
	RegisterWindow         time.Duration
	PasskeyNetworkCapacity int64
	PasskeyWindow          time.Duration
	MFANetworkCapacity     int64
	MFATargetCapacity      int64
	MFAWindow              time.Duration
	Challenge              *abuse.ChallengeDefinition
}

func Definition(policy Policy) abuse.Definition {
	if policy.LoginAccountCapacity <= 0 {
		policy.LoginAccountCapacity = 20
	}
	if policy.LoginNetworkCapacity <= 0 {
		policy.LoginNetworkCapacity = 50
	}
	if policy.LoginWindow <= 0 {
		policy.LoginWindow = 15 * time.Minute
	}
	if policy.RegisterCapacity <= 0 {
		policy.RegisterCapacity = 5
	}
	if policy.RegisterWindow <= 0 {
		policy.RegisterWindow = 10 * time.Minute
	}
	if policy.PasskeyNetworkCapacity <= 0 {
		policy.PasskeyNetworkCapacity = 30
	}
	if policy.PasskeyWindow <= 0 {
		policy.PasskeyWindow = 10 * time.Minute
	}
	if policy.MFANetworkCapacity <= 0 {
		policy.MFANetworkCapacity = 30
	}
	if policy.MFATargetCapacity <= 0 {
		policy.MFATargetCapacity = 5
	}
	if policy.MFAWindow <= 0 {
		policy.MFAWindow = 10 * time.Minute
	}
	challengeAt := int64(0)
	if policy.Challenge != nil {
		challengeAt = 6
		if challengeAt >= policy.LoginAccountCapacity {
			challengeAt = max(1, policy.LoginAccountCapacity-1)
		}
	}
	return abuse.Definition{
		Version: 1, Consumer: "identity",
		Actions: []abuse.ActionDefinition{
			{
				Key: ActionRegister,
				Required: abuse.SignalRequirements{
					Network: abuse.Required, Target: abuse.Required,
				},
				Meters: []abuse.MeterDefinition{
					{
						ID: "identity.register.network", Slot: abuse.SlotNetwork,
						Algorithm: abuse.TokenBucket(
							policy.RegisterCapacity, policy.RegisterCapacity, policy.RegisterWindow,
						),
					},
					{
						ID: "identity.register.target", Slot: abuse.SlotTarget,
						Algorithm: abuse.FixedWindow(3, 24*time.Hour),
					},
				},
			},
			{
				Key: ActionLogin,
				Required: abuse.SignalRequirements{
					Network: abuse.Required, Target: abuse.Required,
				},
				Meters: []abuse.MeterDefinition{
					{
						ID: "identity.login.network_requests", Slot: abuse.SlotNetwork,
						Algorithm: abuse.TokenBucket(
							policy.LoginNetworkCapacity,
							policy.LoginNetworkCapacity,
							policy.LoginWindow,
						),
					},
					{
						ID: "identity.login.network_failures", Slot: abuse.SlotNetwork,
						Mode:      abuse.MeterOutcome,
						Algorithm: abuse.SlidingWindow(policy.LoginNetworkCapacity, policy.LoginWindow),
						ChargeOn:  []abuse.OutcomeKey{"credentials_rejected"},
					},
					{
						ID: "identity.login.target_failures", Slot: abuse.SlotTarget,
						Mode:        abuse.MeterOutcome,
						Algorithm:   abuse.SlidingWindow(policy.LoginAccountCapacity, policy.LoginWindow),
						ChallengeAt: challengeAt,
						ChargeOn:    []abuse.OutcomeKey{"credentials_rejected"},
						ResetOn:     []abuse.OutcomeKey{"authenticated"},
					},
				},
				Resolution: &abuse.ResolutionDefinition{
					Outcomes:       []abuse.OutcomeKey{"authenticated", "credentials_rejected"},
					DefaultOutcome: "credentials_rejected",
					PendingTTL:     time.Minute,
				},
				Challenge: policy.Challenge,
			},
			{
				Key: ActionPasskeyCeremony,
				Required: abuse.SignalRequirements{
					Network: abuse.Required,
				},
				Meters: []abuse.MeterDefinition{
					{
						ID: "identity.passkey_ceremony.network", Slot: abuse.SlotNetwork,
						Algorithm: abuse.TokenBucket(
							policy.PasskeyNetworkCapacity,
							policy.PasskeyNetworkCapacity,
							policy.PasskeyWindow,
						),
					},
				},
			},
			{
				Key: ActionMFAVerification,
				Required: abuse.SignalRequirements{
					Network: abuse.Required, Target: abuse.Required,
				},
				Meters: []abuse.MeterDefinition{
					{
						ID: "identity.mfa.network_requests", Slot: abuse.SlotNetwork,
						Algorithm: abuse.TokenBucket(
							policy.MFANetworkCapacity*2,
							policy.MFANetworkCapacity*2,
							policy.MFAWindow,
						),
					},
					{
						ID: "identity.mfa.network_failures", Slot: abuse.SlotNetwork,
						Mode: abuse.MeterOutcome,
						Algorithm: abuse.SlidingWindow(
							policy.MFANetworkCapacity, policy.MFAWindow,
						),
						ChargeOn: []abuse.OutcomeKey{"verification_rejected"},
					},
					{
						ID: "identity.mfa.transaction_failures", Slot: abuse.SlotTarget,
						Mode: abuse.MeterOutcome,
						Algorithm: abuse.SlidingWindow(
							policy.MFATargetCapacity, policy.MFAWindow,
						),
						ChargeOn: []abuse.OutcomeKey{"verification_rejected"},
						ResetOn:  []abuse.OutcomeKey{"verified"},
					},
				},
				Resolution: &abuse.ResolutionDefinition{
					Outcomes: []abuse.OutcomeKey{
						"verified", "verification_rejected", "verification_aborted",
					},
					DefaultOutcome: "verification_aborted",
					PendingTTL:     time.Minute,
				},
			},
		},
	}
}

type Actions struct {
	Register        abuse.Action
	Login           abuse.Action
	PasskeyCeremony abuse.Action
	MFAVerification abuse.Action
}

func Bind(module abuse.Module) (Actions, error) {
	register, err := module.Action(ActionRegister)
	if err != nil {
		return Actions{}, err
	}
	login, err := module.Action(ActionLogin)
	if err != nil {
		return Actions{}, err
	}
	passkeyCeremony, err := module.Action(ActionPasskeyCeremony)
	if err != nil {
		return Actions{}, err
	}
	mfaVerification, err := module.Action(ActionMFAVerification)
	if err != nil {
		return Actions{}, err
	}
	return Actions{
		Register: register, Login: login, PasskeyCeremony: passkeyCeremony,
		MFAVerification: mfaVerification,
	}, nil
}

func NetworkPrefix(value string) (netip.Prefix, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Prefix{}, err
	}
	address = address.Unmap()
	bits := address.BitLen()
	if address.Is6() {
		bits = 64
	}
	return netip.PrefixFrom(address, bits).Masked(), nil
}

func Admit(
	ctx context.Context,
	action abuse.Action,
	attemptID string,
	network netip.Prefix,
	target string,
	proof string,
) (abuse.Admission, error) {
	input := abuse.Input{
		ID:      abuse.AttemptID(attemptID),
		Signals: abuse.Signals{Network: network, Target: target},
	}
	if strings.TrimSpace(proof) != "" {
		input.Proof = &abuse.Proof{Kind: "turnstile", Token: proof}
	}
	return action.Admit(ctx, input)
}
