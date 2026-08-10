//go:build integration

package devprovisioning

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestClientsOnlyReconcileIsIdempotentAndDoesNotChangeAccounts(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("IDENTITY_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("IDENTITY_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	siteID := "workspace-site-test-" + suffix
	serviceID := "workspace-service-test-" + suffix
	unmanagedID := "operator-client-test-" + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM oidc_clients WHERE id = ANY($1)`,
			pq.Array([]string{siteID, serviceID, unmanagedID}),
		)
	})

	var identitiesBefore int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM identities`).Scan(&identitiesBefore); err != nil {
		t.Fatal(err)
	}
	declared := Declaration{
		SiteClients: []SiteClient{{
			ID: siteID, RedirectURIs: []string{"http://localhost:3006/auth/callback"},
			PostLogoutRedirectURIs: []string{"http://localhost:3006/"},
			Audiences:              []string{"identity-api"},
		}},
		ServiceClients: []ServiceClient{{
			ID: serviceID, Secret: "workspace-integration-secret", SecretRef: "env:TEST_SECRET",
			Audience: "identity-api", Scopes: []string{"identity:test"},
		}},
	}
	if _, err := Reconcile(ctx, db, declared); err != nil {
		t.Fatal(err)
	}
	var firstHash string
	if err := db.QueryRowContext(ctx,
		`SELECT secret_hash FROM oidc_clients WHERE id=$1`, serviceID,
	).Scan(&firstHash); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(ctx, db, declared); err != nil {
		t.Fatal(err)
	}
	var identitiesAfter int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM identities`).Scan(&identitiesAfter); err != nil {
		t.Fatal(err)
	}
	if identitiesAfter != identitiesBefore {
		t.Fatalf("identity count changed: before=%d after=%d", identitiesBefore, identitiesAfter)
	}
	var secondHash, siteOwner, serviceOwner string
	if err := db.QueryRowContext(ctx,
		`SELECT secret_hash, managed_by FROM oidc_clients WHERE id=$1`, serviceID,
	).Scan(&secondHash, &serviceOwner); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT managed_by FROM oidc_clients WHERE id=$1`, siteID,
	).Scan(&siteOwner); err != nil {
		t.Fatal(err)
	}
	if secondHash != firstHash || siteOwner != "workspace" || serviceOwner != "workspace" {
		t.Fatalf(
			"idempotent state = firstHashSame:%t siteOwner:%q serviceOwner:%q",
			secondHash == firstHash,
			siteOwner,
			serviceOwner,
		)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO oidc_clients (id, managed_by) VALUES ($1, '')`, unmanagedID); err != nil {
		t.Fatal(err)
	}
	_, err = Reconcile(ctx, db, Declaration{SiteClients: []SiteClient{{
		ID: unmanagedID, RedirectURIs: []string{"http://localhost:3007/auth/callback"},
		PostLogoutRedirectURIs: []string{"http://localhost:3007/"},
		Audiences:              []string{"identity-api"},
	}}})
	if err == nil || !strings.Contains(err.Error(), "not owned by local Workspace provisioning") {
		t.Fatalf("unmanaged client error = %v", err)
	}
}
