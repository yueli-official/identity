// Command devseed reconciles Identity-owned local development fixtures.
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
	raw := strings.TrimSpace(os.Getenv("IDENTITY_DEV_SEED"))
	if databaseURL == "" || raw == "" {
		fatal("IDENTITY_DATABASE_URL and IDENTITY_DEV_SEED are required")
	}
	declared, err := parseSeed(raw)
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
	result, err := reconcile(ctx, db, declared)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf(
		"identity development seed reconciled (account=%s, siteClients=%d, serviceClients=%d)\n",
		result.AccountEmail,
		result.SiteClients,
		result.ServiceClients,
	)
}

func parseSeed(raw string) (devprovisioning.Declaration, error) {
	return devprovisioning.Parse(raw, "IDENTITY_DEV_SEED", devprovisioning.FullSeed)
}

func reconcile(
	ctx context.Context,
	db *sql.DB,
	declared devprovisioning.Declaration,
) (devprovisioning.Result, error) {
	return devprovisioning.Reconcile(ctx, db, declared)
}

func fatal(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, "devseed: "+format+"\n", arguments...)
	os.Exit(1)
}
