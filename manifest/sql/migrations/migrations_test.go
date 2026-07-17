package migrations_test

import (
	"os"
	"strings"
	"testing"
)

func TestInitMigrationHasCoreTables(t *testing.T) {
	up, err := os.ReadFile("0001_init.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE identities",
		"CREATE TABLE user_profiles",
		"CREATE TABLE credentials_password",
		"UNIQUE",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("up migration missing %q", want)
		}
	}
	if _, err := os.Stat("0001_init.down.sql"); err != nil {
		t.Errorf("down migration missing: %v", err)
	}
}

func TestOIDCMigrationHasCoreTables(t *testing.T) {
	up, err := os.ReadFile("0002_oidc.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE oidc_clients",
		"CREATE TABLE oidc_signing_keys",
		"INSERT INTO oidc_clients",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0002 up missing %q", want)
		}
	}
	if _, err := os.Stat("0002_oidc.down.sql"); err != nil {
		t.Errorf("0002 down missing: %v", err)
	}
}

func TestOIDCSessionMigrationHasCoreTables(t *testing.T) {
	up, err := os.ReadFile("0003_oidc_sessions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE oidc_oauth_requests",
		"CREATE TABLE oidc_refresh_tokens",
		"offline_access",
		"refresh_token",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0003 up missing %q", want)
		}
	}
	if _, err := os.Stat("0003_oidc_sessions.down.sql"); err != nil {
		t.Errorf("0003 down missing: %v", err)
	}
}

func TestIdentitySessionMigrationHasDurableLoginSessions(t *testing.T) {
	up, err := os.ReadFile("0011_identity_sessions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE identity_sessions",
		"identity_id",
		"expires_at",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0011 up missing %q", want)
		}
	}
	if _, err := os.Stat("0011_identity_sessions.down.sql"); err != nil {
		t.Errorf("0011 down missing: %v", err)
	}
}

func TestGuestSessionMigrationHasDurableClaimableSessions(t *testing.T) {
	up, err := os.ReadFile("0015_guest_sessions.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE guest_sessions",
		"token_hash",
		"client_id",
		"expires_at",
		"claimed_identity_id",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0015 up missing %q", want)
		}
	}
	if _, err := os.Stat("0015_guest_sessions.down.sql"); err != nil {
		t.Errorf("0015 down missing: %v", err)
	}
}
