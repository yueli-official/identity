//go:build integration

package dao_test

import (
	"context"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/repo"
)

func newDB(t *testing.T) gdb.DB {
	link := os.Getenv("TEST_PG_LINK") // e.g. pgsql:user:pass@tcp(127.0.0.1:5432)/identity_test
	if link == "" {
		t.Skip("TEST_PG_LINK not set; skipping pg integration test (requires migrations applied)")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: link})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPGCreateAndGet(t *testing.T) {
	db := newDB(t)
	d := dao.NewPG(db)
	ctx := context.Background()
	id, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "it@pg.com", PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.GetByEmail(ctx, "it@pg.com")
	if err != nil || got.ID != id.ID {
		t.Fatalf("get by email: %v %#v", err, got)
	}
	if _, err := d.CreateIdentityWithProfile(ctx, repo.NewIdentityInput{Email: "it@pg.com", PasswordHash: "h"}); err != repo.ErrEmailTaken {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
	_, _ = db.Model("identities").Ctx(ctx).Where("email", "it@pg.com").Delete()
}
