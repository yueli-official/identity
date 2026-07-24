package identityabuse

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/foundation/go/abuse"
)

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
