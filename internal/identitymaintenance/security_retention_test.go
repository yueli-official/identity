package identitymaintenance

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestSecurityRetentionRejectsMissingDatabaseAndUnboundedBatch(t *testing.T) {
	if _, err := (SecurityRetention{}).RunOnce(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "database is required") {
		t.Fatalf("missing database error = %v", err)
	}
	if _, err := (SecurityRetention{
		DB: &sql.DB{}, BatchSize: 10_001,
	}).RunOnce(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "must not exceed") {
		t.Fatalf("unbounded batch error = %v", err)
	}
}
