// Command devprovision reconciles Consumer-owned OIDC clients in a local
// shared Identity Provider without changing account fixtures or Provider
// runtime configuration.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/yueli-official/identity/internal/devprovisioning"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("IDENTITY_DATABASE_URL"))
	raw := strings.TrimSpace(os.Getenv("IDENTITY_DEV_PROVISIONING"))
	if databaseURL == "" || raw == "" {
		fatal("IDENTITY_DATABASE_URL and IDENTITY_DEV_PROVISIONING are required")
	}
	declared, err := devprovisioning.Parse(
		raw,
		"IDENTITY_DEV_PROVISIONING",
		devprovisioning.ClientsOnly,
	)
	if err != nil {
		fatal("%v", err)
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		fatal("open database: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		fatal("connect database: %v", err)
	}
	result, err := devprovisioning.Reconcile(ctx, db, declared)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf(
		"identity development clients provisioned (siteClients=%d, serviceClients=%d)\n",
		result.SiteClients,
		result.ServiceClients,
	)
}

func fatal(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, "devprovision: "+format+"\n", arguments...)
	os.Exit(1)
}
