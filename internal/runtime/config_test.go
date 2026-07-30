package runtime

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/os/gcfg"
)

func TestEnvironmentAdapterOverridesScalarAndNestedConfig(t *testing.T) {
	base, err := gcfg.NewAdapterContent(`
database:
  default:
    host: file-db
test:
  origins:
    - https://file.example
`)
	if err != nil {
		t.Fatal(err)
	}
	config := gcfg.NewWithAdapter(&environmentAdapter{base: base})
	t.Setenv("GF_DATABASE_DEFAULT_HOST", "container-db")
	t.Setenv("GF_TEST_ORIGINS", `["https://env.example"]`)

	host := config.MustGet(context.Background(), "database.default.host").String()
	if host != "container-db" {
		t.Fatalf("scalar host = %q", host)
	}
	database := config.MustGet(context.Background(), "database").Map()
	defaultConfig, ok := database["default"].(map[string]any)
	if !ok || defaultConfig["host"] != "container-db" {
		t.Fatalf("nested database config = %#v", database)
	}
	all, err := config.Data(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	databaseFromData := all["database"].(map[string]any)
	defaultFromData := databaseFromData["default"].(map[string]any)
	if defaultFromData["host"] != "container-db" {
		t.Fatalf("Data() database config = %#v", databaseFromData)
	}
	origins := config.MustGet(context.Background(), "test.origins").Strings()
	if len(origins) != 1 || origins[0] != "https://env.example" {
		t.Fatalf("origins = %#v", origins)
	}
}
