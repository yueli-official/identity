//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"

	"github.com/yueli-official/identity/internal/identitymaintenance"
)

func TestMigrationUpDownUpLifecycle(t *testing.T) {
	host := os.Getenv("IDENTITY_MIGRATION_PG_HOST")
	if host == "" {
		t.Skip("set IDENTITY_MIGRATION_PG_HOST to run the Identity migration lifecycle test")
	}
	port := envOr("IDENTITY_MIGRATION_PG_PORT", "5432")
	user := envOr("IDENTITY_MIGRATION_PG_USER", "postgres")
	password := os.Getenv("IDENTITY_MIGRATION_PG_PASSWORD")

	admin, err := sql.Open("postgres", migrationDSN(host, port, user, password, "postgres"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	database := fmt.Sprintf("identity_migration_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(database)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, database)
		_, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(database))
	})

	db, err := sql.Open("postgres", migrationDSN(host, port, user, password, database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	up := migrationFiles(t, "*.up.sql", false)
	down := migrationFiles(t, "*.down.sql", true)
	if len(up) != len(down) {
		t.Fatalf("migration pairs: up=%d down=%d", len(up), len(down))
	}

	applyMigrationFiles(t, db, up)
	assertTableExists(t, db, "identities", true)
	assertTableExists(t, db, "authentication_ceremonies", true)
	assertTableExists(t, db, "authentication_transactions", true)
	assertTableExists(t, db, "step_up_proof_uses", true)
	assertTableExists(t, db, "publisher_attestations", true)
	assertTableExists(t, db, "github_identity_bindings", true)

	applyMigrationFiles(t, db, down)
	assertTableExists(t, db, "identities", false)
	assertTableExists(t, db, "authentication_ceremonies", false)
	assertTableExists(t, db, "authentication_transactions", false)
	assertTableExists(t, db, "step_up_proof_uses", false)
	assertTableExists(t, db, "publisher_attestations", false)
	assertTableExists(t, db, "github_identity_bindings", false)

	applyMigrationFiles(t, db, up)
	assertTableExists(t, db, "identities", true)
	assertTableExists(t, db, "step_up_proof_uses", true)
	assertTableExists(t, db, "publisher_attestations", true)
	assertTableExists(t, db, "github_identity_bindings", true)
	assertSecurityRetention(t, db)
}

func assertSecurityRetention(t *testing.T, db *sql.DB) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	oldJTI := uuid.NewString()
	freshJTI := uuid.NewString()
	if _, err := db.Exec(`
INSERT INTO step_up_proof_uses(jti, expires_at, consumed_at)
VALUES ($1, $2, $3), ($4, $5, $3)
`, oldJTI, now.Add(-time.Minute), now, freshJTI, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err := (identitymaintenance.SecurityRetention{
		DB: db, Retention: time.Hour, BatchSize: 10, Clock: func() time.Time { return now },
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProofUses != 1 {
		t.Fatalf("deleted proof uses = %d, want 1", result.ProofUses)
	}
	identityID := uuid.NewString()
	if _, err := db.Exec(
		`INSERT INTO identities(id, email) VALUES ($1, $2)`,
		identityID, "github-retention@example.test",
	); err != nil {
		t.Fatal(err)
	}
	oldAttemptID := uuid.NewString()
	freshAttemptID := uuid.NewString()
	if _, err := db.Exec(`
INSERT INTO github_binding_attempts(
    id, state_digest, identity_id, session_digest, verifier_ciphertext,
    return_to, expires_at, created_at
) VALUES
($1, $2, $3, $4, 'cipher-old', '/', $5, $6),
($7, $8, $3, $9, 'cipher-fresh', '/', $10, $6)
`,
		oldAttemptID, strings.Repeat("a", 64), identityID, strings.Repeat("b", 64),
		now.Add(-time.Minute), now,
		freshAttemptID, strings.Repeat("c", 64), strings.Repeat("d", 64),
		now.Add(time.Minute),
	); err != nil {
		t.Fatal(err)
	}
	secondResult, err := (identitymaintenance.SecurityRetention{
		DB: db, Retention: time.Hour, BatchSize: 10, Clock: func() time.Time { return now },
	}).RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if secondResult.BindingAttempts != 1 {
		t.Fatalf("deleted binding attempts = %d, want 1", secondResult.BindingAttempts)
	}
	var remaining int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM step_up_proof_uses WHERE jti IN ($1, $2)`, oldJTI, freshJTI,
	).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining proof uses = %d, want 1", remaining)
	}
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM github_binding_attempts WHERE id IN ($1, $2)`,
		oldAttemptID, freshAttemptID,
	).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining binding attempts = %d, want 1", remaining)
	}
}

func migrationFiles(t *testing.T, pattern string, reverse bool) []string {
	t.Helper()
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if reverse {
		for left, right := 0, len(files)-1; left < right; left, right = left+1, right-1 {
			files[left], files[right] = files[right], files[left]
		}
	}
	return files
}

func applyMigrationFiles(t *testing.T, db *sql.DB, files []string) {
	t.Helper()
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(file), err)
		}
	}
}

func assertTableExists(t *testing.T, db *sql.DB, table string, want bool) {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("table %s exists=%v, want %v", table, exists, want)
	}
}

func migrationDSN(host, port, user, password, database string) string {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   database,
	}
	query := dsn.Query()
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
