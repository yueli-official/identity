//go:build integration

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2" // register the go-redis adapter

	"platform/services/identity/internal/cache"
	"platform/services/identity/internal/model"
)

func newRedis(t *testing.T) *cache.Redis {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set; skipping redis integration test")
	}
	c, err := gredis.New(&gredis.Config{Address: addr})
	if err != nil {
		t.Fatal(err)
	}
	return cache.NewRedis(c)
}

func TestRedisSessionRoundTrip(t *testing.T) {
	r := newRedis(t)
	ctx := context.Background()
	s := model.Session{ID: "it-sess-1", IdentityID: "it-u-1"}
	if err := r.CreateSession(ctx, s, time.Minute); err != nil {
		t.Fatal(err)
	}
	defer r.DeleteSession(ctx, s.ID)
	got, err := r.GetSession(ctx, s.ID)
	if err != nil || got.IdentityID != "it-u-1" {
		t.Fatalf("roundtrip: %v %#v", err, got)
	}
	list, _ := r.ListSessionsByIdentity(ctx, "it-u-1")
	if len(list) != 1 {
		t.Fatalf("want 1, got %d", len(list))
	}
}
