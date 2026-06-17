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
