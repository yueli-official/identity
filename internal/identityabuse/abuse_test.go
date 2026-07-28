package identityabuse

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/foundation/go/abuse"
)

func TestDefinitionVersionAdvancesWithPublisherAction(t *testing.T) {
	definition := Definition(Policy{})
	if got, want := definition.Version, uint64(2); got != want {
		t.Fatalf("definition version = %d, want %d after adding publisher action", got, want)
	}
}

func TestRegistrationBudgetRejectsAtomically(t *testing.T) {
	module, err := abuse.NewMemory(
		abuse.MustCompile(Definition(Policy{
			RegisterCapacity: 2,
			RegisterWindow:   time.Hour,
		})),
		abuse.MemoryOptions{Secret: []byte("identity-abuse-test-secret-at-least-32-bytes")},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := Bind(module)
	if err != nil {
		t.Fatal(err)
	}
	network, err := NetworkPrefix("2001:db8:1:2::9")
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []abuse.Disposition{
		abuse.DispositionAllow,
		abuse.DispositionAllow,
		abuse.DispositionReject,
	} {
		got, err := Admit(
			context.Background(), actions.Register,
			"registration-"+string(rune('a'+index)), network, "person@example.com", "",
		)
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != want {
			t.Fatalf("attempt %d: got %q, want %q", index+1, got.Disposition, want)
		}
	}
}

func TestNetworkPrefixMasksIPv6ButNotIPv4(t *testing.T) {
	ipv6, err := NetworkPrefix("2001:db8:1:2::99")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ipv6.String(), "2001:db8:1:2::/64"; got != want {
		t.Fatalf("IPv6 prefix got %q, want %q", got, want)
	}
	ipv4, err := NetworkPrefix("192.0.2.9")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ipv4.String(), "192.0.2.9/32"; got != want {
		t.Fatalf("IPv4 prefix got %q, want %q", got, want)
	}
}

func TestPasskeyCeremonyBudgetIsNetworkBound(t *testing.T) {
	module, err := abuse.NewMemory(
		abuse.MustCompile(Definition(Policy{
			PasskeyNetworkCapacity: 2,
			PasskeyWindow:          time.Hour,
		})),
		abuse.MemoryOptions{Secret: []byte("identity-abuse-test-secret-at-least-32-bytes")},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := Bind(module)
	if err != nil {
		t.Fatal(err)
	}
	network, err := NetworkPrefix("192.0.2.9")
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []abuse.Disposition{
		abuse.DispositionAllow,
		abuse.DispositionAllow,
		abuse.DispositionReject,
	} {
		got, err := actions.PasskeyCeremony.Admit(context.Background(), abuse.Input{
			ID:      abuse.AttemptID("passkey-ceremony-" + string(rune('a'+index))),
			Signals: abuse.Signals{Network: network},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Disposition != want {
			t.Fatalf("attempt %d: got %q, want %q", index+1, got.Disposition, want)
		}
	}
}

func TestPublisherIssueBudgetIsActorBoundAndReplaySafe(t *testing.T) {
	module, err := abuse.NewMemory(
		abuse.MustCompile(Definition(Policy{
			PublisherActorCapacity: 2,
			PublisherWindow:        time.Hour,
		})),
		abuse.MemoryOptions{Secret: []byte("identity-abuse-test-secret-at-least-32-bytes")},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := Bind(module)
	if err != nil {
		t.Fatal(err)
	}
	network, _ := NetworkPrefix("192.0.2.29")
	input := abuse.Input{
		ID: abuse.AttemptID("publisher-a"),
		Signals: abuse.Signals{
			Network: network,
			Target:  "publisher-subject",
		},
	}
	first, err := actions.PublisherIssue.Admit(context.Background(), input)
	if err != nil || first.Disposition != abuse.DispositionAllow || first.Replay {
		t.Fatalf("first publisher admission = %+v, %v", first, err)
	}
	replay, err := actions.PublisherIssue.Admit(context.Background(), input)
	if err != nil || replay.Disposition != abuse.DispositionAllow || !replay.Replay {
		t.Fatalf("publisher replay = %+v, %v", replay, err)
	}
	for _, id := range []string{"publisher-b", "publisher-c"} {
		input.ID = abuse.AttemptID(id)
		got, admitErr := actions.PublisherIssue.Admit(context.Background(), input)
		if admitErr != nil {
			t.Fatal(admitErr)
		}
		want := abuse.DispositionAllow
		if id == "publisher-c" {
			want = abuse.DispositionReject
		}
		if got.Disposition != want {
			t.Fatalf("%s disposition = %q, want %q", id, got.Disposition, want)
		}
	}
}

func TestMFAVerificationFailureBudgetSpansTransactionsByNetwork(t *testing.T) {
	module, err := abuse.NewMemory(
		abuse.MustCompile(Definition(Policy{
			MFANetworkCapacity: 2, MFATargetCapacity: 5, MFAWindow: time.Hour,
		})),
		abuse.MemoryOptions{Secret: []byte("identity-abuse-test-secret-at-least-32-bytes")},
	)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := Bind(module)
	if err != nil {
		t.Fatal(err)
	}
	network, _ := NetworkPrefix("192.0.2.19")
	for index := 0; index < 2; index++ {
		admission, err := Admit(
			context.Background(), actions.MFAVerification,
			"mfa-"+string(rune('a'+index)), network,
			"transaction-"+string(rune('a'+index)), "",
		)
		if err != nil || admission.Disposition != abuse.DispositionAllow {
			t.Fatalf("admission %d = %+v, %v", index, admission, err)
		}
		if err := actions.MFAVerification.Resolve(
			context.Background(), admission.Receipt, "verification_rejected",
		); err != nil {
			t.Fatal(err)
		}
	}
	third, err := Admit(
		context.Background(), actions.MFAVerification,
		"mfa-c", network, "transaction-c", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if third.Disposition != abuse.DispositionReject {
		t.Fatalf("third MFA admission = %+v, want reject", third)
	}
}
