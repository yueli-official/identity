//go:build integration

package stepup

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func TestPostgreSQLReplayStoreAtomicallyConsumesJTI(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := PostgreSQLReplayStore{DB: db}
	jti := uuid.NewString()
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM step_up_proof_uses WHERE jti = $1", jti)
	})
	first, err := store.Consume(context.Background(), jti, time.Now().Add(time.Minute))
	if err != nil || !first {
		t.Fatalf("first Consume() = %v, %v; want true", first, err)
	}
	second, err := store.Consume(context.Background(), jti, time.Now().Add(time.Minute))
	if err != nil || second {
		t.Fatalf("second Consume() = %v, %v; want false", second, err)
	}
}

func TestPostgreSQLReplayStoreAllowsExactlyOneConcurrentConsumer(t *testing.T) {
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := PostgreSQLReplayStore{DB: db}
	jti := uuid.NewString()
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM step_up_proof_uses WHERE jti = $1", jti)
	})

	const contenders = 32
	start := make(chan struct{})
	results := make(chan bool, contenders)
	failures := make(chan error, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			consumed, consumeErr := store.Consume(
				context.Background(), jti, time.Now().Add(time.Minute),
			)
			if consumeErr != nil {
				failures <- consumeErr
				return
			}
			results <- consumed
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(failures)

	for failure := range failures {
		t.Errorf("Consume() error = %v", failure)
	}
	winners := 0
	for consumed := range results {
		if consumed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent Consume() winners = %d, want 1", winners)
	}
}
