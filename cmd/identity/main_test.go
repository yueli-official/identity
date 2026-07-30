package main

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/oidc"
	"github.com/yueli-official/identity/internal/repo"
)

func TestOpenAPIRepositoriesAreHermetic(t *testing.T) {
	dependencies := newRuntimeRepositories(true)
	memory, ok := dependencies.store.(*repo.Memory)
	if !ok {
		t.Fatalf("OpenAPI store = %T, want *repo.Memory", dependencies.store)
	}
	if dependencies.clients != memory || dependencies.signingKeys != memory || dependencies.audit != memory {
		t.Fatal("OpenAPI repositories must share the hermetic memory store")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := oidc.NewManager(ctx, dependencies.signingKeys); err != nil {
		t.Fatalf("initialize OpenAPI signing keys without PostgreSQL: %v", err)
	}
}

func TestPATHMACSecret(t *testing.T) {
	t.Run("dedicated secret wins", func(t *testing.T) {
		if got := patHMACSecret("  dedicated-secret  ", "global-secret"); got != "dedicated-secret" {
			t.Fatalf("patHMACSecret() = %q", got)
		}
	})
	t.Run("global secret is the safe default", func(t *testing.T) {
		if got := patHMACSecret("", "global-secret"); got != "global-secret" {
			t.Fatalf("patHMACSecret() = %q", got)
		}
	})
}
