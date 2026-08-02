package githubbinding

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"
)

type fakeProvider struct {
	account  Account
	verifier string
	revoked  string
}

func (provider *fakeProvider) AuthorizationURL(state, challenge string) string {
	query := url.Values{"state": {state}, "code_challenge": {challenge}}
	return "https://github.test/authorize?" + query.Encode()
}

func (provider *fakeProvider) ExchangeCode(
	_ context.Context,
	_ string,
	verifier string,
) (string, error) {
	provider.verifier = verifier
	return "ghu_test", nil
}

func (provider *fakeProvider) AuthenticatedUser(context.Context, string) (Account, error) {
	return provider.account, nil
}

func (provider *fakeProvider) RevokeAccessToken(_ context.Context, token string) error {
	provider.revoked = token
	return nil
}

func newTestModule(t *testing.T, store Store, provider Provider, now time.Time) *Module {
	t.Helper()
	module, err := New(Config{
		Store: store, Provider: provider,
		CipherSecret: []byte("0123456789abcdef0123456789abcdef"),
		AttemptTTL:   time.Minute, Now: func() time.Time { return now },
		ResolvePublisherSubject: func(_ context.Context, identityID string) (string, error) { return identityID, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func TestBindingFlowUsesOneTimeSessionBoundPKCEAndStableAccountID(t *testing.T) {
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	provider := &fakeProvider{account: Account{
		AccountID: "583231", NodeID: "U_node", Login: "first-login",
	}}
	module := newTestModule(t, store, provider, now)

	started, err := module.Begin(context.Background(), "identity-1", "session-1", "/settings/connections")
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizationURL.Query().Get("state")
	if state == "" || authorizationURL.Query().Get("code_challenge") == "" {
		t.Fatalf("authorization URL missing state/PKCE: %s", started.AuthorizationURL)
	}
	if _, err := module.Complete(
		context.Background(), state, "other-session", "code",
	); !errors.Is(err, ErrInvalidAttempt) {
		t.Fatalf("cross-session callback error = %v", err)
	}

	// The wrong-session attempt did not consume the record.
	completed, err := module.Complete(
		context.Background(), state, "session-1", "code",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.Created || completed.Binding.ProviderAccountID != "583231" ||
		completed.Binding.IdentityID != "identity-1" {
		t.Fatalf("binding = %+v", completed)
	}
	if provider.verifier == "" || provider.revoked != "ghu_test" {
		t.Fatalf("PKCE verifier/revocation missing: %+v", provider)
	}
	if _, err := module.Complete(
		context.Background(), state, "session-1", "code",
	); !errors.Is(err, ErrInvalidAttempt) {
		t.Fatalf("replayed callback error = %v", err)
	}
}

func TestBindingConflictRenameUnbindAndRebindPreserveHistory(t *testing.T) {
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	provider := &fakeProvider{account: Account{AccountID: "42", Login: "old-login"}}
	first := newTestModule(t, store, provider, now)

	complete := func(module *Module, identityID, sessionID string) CompleteResult {
		t.Helper()
		started, err := module.Begin(context.Background(), identityID, sessionID, "/account")
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := url.Parse(started.AuthorizationURL)
		result, err := module.Complete(
			context.Background(), parsed.Query().Get("state"), sessionID, "code",
		)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	created := complete(first, "identity-1", "session-1")
	provider.account.Login = "new-login"
	renamed := complete(first, "identity-1", "session-1")
	if !renamed.Renamed || renamed.Binding.ID != created.Binding.ID ||
		renamed.Binding.Login != "new-login" {
		t.Fatalf("rename = %+v", renamed)
	}

	started, _ := first.Begin(context.Background(), "identity-2", "session-2", "/account")
	parsed, _ := url.Parse(started.AuthorizationURL)
	if _, err := first.Complete(
		context.Background(), parsed.Query().Get("state"), "session-2", "code",
	); !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("conflicting bind error = %v", err)
	}

	if _, err := first.Unbind(context.Background(), "identity-1", created.Binding.ID); err != nil {
		t.Fatal(err)
	}
	rebound := complete(first, "identity-2", "session-2")
	if rebound.Binding.ID == created.Binding.ID {
		t.Fatal("rebind rewrote historical binding")
	}
	history, err := first.List(context.Background(), "identity-1")
	if err != nil || len(history) != 1 || history[0].Status != StatusUnbound {
		t.Fatalf("old history = %+v, err=%v", history, err)
	}
}

func TestAuthorizationRevokedBlocksFutureLookupWithoutDeletingHistory(t *testing.T) {
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	bound, err := store.Bind(
		context.Background(), "identity-1",
		Account{AccountID: "99", Login: "before"}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	module := newTestModule(t, store, &fakeProvider{}, now)
	blocked, err := module.AuthorizationRevoked(context.Background(), "99", "after")
	if err != nil || len(blocked) != 1 || blocked[0].Status != StatusBlocked {
		t.Fatalf("blocked = %+v, err=%v", blocked, err)
	}
	if _, err := store.FindActiveByAccount(context.Background(), "99"); !errors.Is(err, ErrBindingInactive) {
		t.Fatalf("lookup after revoke error = %v", err)
	}
	history, _ := store.ListByIdentity(context.Background(), "identity-1")
	if len(history) != 1 || history[0].ID != bound.Binding.ID {
		t.Fatalf("history = %+v", history)
	}
}
