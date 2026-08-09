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

func TestAuthenticationContextMigrationPreservesServerObservedFacts(t *testing.T) {
	up, err := os.ReadFile("0020_authentication_context.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE authentication_events",
		"authenticated_at",
		"methods",
		"factor_classes",
		"assurance_level",
		"assurance_profile",
		"user_verified",
		"phishing_resistant",
		"recovery",
		"authentication_event_id",
		"ARRAY['legacy']",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0020 up missing %q", want)
		}
	}
	if _, err := os.Stat("0020_authentication_context.down.sql"); err != nil {
		t.Errorf("0020 down migration missing: %v", err)
	}
}

func TestPasskeyMigrationPersistsCredentialAndCeremonyState(t *testing.T) {
	up, err := os.ReadFile("0021_passkeys.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE webauthn_users",
		"CREATE TABLE webauthn_credentials",
		"CREATE TABLE authentication_ceremonies",
		"UNIQUE (rp_id, credential_id)",
		"user_verified_at_registration",
		"backup_eligible",
		"backup_state",
		"sign_count",
		"challenge_digest",
		"library_state",
		"consumed_at",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0021 up missing %q", want)
		}
	}
	if _, err := os.Stat("0021_passkeys.down.sql"); err != nil {
		t.Errorf("0021 down migration missing: %v", err)
	}
}

func TestMFARecoveryAndStepUpMigrationOwnsSecurityState(t *testing.T) {
	up, err := os.ReadFile("0022_mfa_recovery_step_up.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE authentication_policies",
		"second_factor_required",
		"CREATE TABLE totp_authenticators",
		"secret_ciphertext",
		"last_used_step",
		"CREATE TABLE recovery_code_sets",
		"CREATE TABLE recovery_codes",
		"code_digest",
		"consumed_at",
		"CREATE TABLE authentication_transactions",
		"resource_digest",
		"requirement",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0022 up missing %q", want)
		}
	}
	if _, err := os.Stat("0022_mfa_recovery_step_up.down.sql"); err != nil {
		t.Errorf("0022 down migration missing: %v", err)
	}
}

func TestStepUpProofReplayMigrationUsesAtomicJTIKey(t *testing.T) {
	up, err := os.ReadFile("0023_step_up_proof_replay.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE step_up_proof_uses",
		"jti         UUID PRIMARY KEY",
		"expires_at  TIMESTAMPTZ NOT NULL",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0023 up missing %q", want)
		}
	}
	if _, err := os.Stat("0023_step_up_proof_replay.down.sql"); err != nil {
		t.Errorf("0023 down migration missing: %v", err)
	}
}

func TestGitHubBindingMigrationSeparatesOneTimeAttemptsAndHistory(t *testing.T) {
	up, err := os.ReadFile("0025_github_identity_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"CREATE TABLE github_binding_attempts",
		"state_digest",
		"session_digest",
		"verifier_ciphertext",
		"consumed_at",
		"CREATE TABLE github_identity_bindings",
		"provider_account_id",
		"login_snapshot",
		"status IN ('active', 'unbound', 'blocked')",
		"erased_at",
		"No FK by design",
		"WHERE status = 'active'",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0025 up missing %q", want)
		}
	}
	if _, err := os.Stat("0025_github_identity_bindings.down.sql"); err != nil {
		t.Errorf("0025 down migration missing: %v", err)
	}
}

func TestPublicUserContractUsesCompactFoundationKeyWithoutSQLGeneration(t *testing.T) {
	up, err := os.ReadFile("0026_user_identity_contract.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	s := string(up)
	for _, want := range []string{
		"ADD COLUMN user_key TEXT NOT NULL",
		"uq_identities_user_key",
		"CHECK (user_key ~ '^[1-9A-HJ-NP-Za-km-z]{8}$')",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("0026 up missing %q", want)
		}
	}
	if strings.Contains(s, "gen_random") {
		t.Error("0026 must not generate identifiers inside SQL")
	}
}
