package identityprivacy

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/yueli-official/foundation/go/privacy"
	"github.com/yueli-official/foundation/go/work"

	"platform/gokit/privacycatalog"
)

func TestCoordinatorFinalizesIdentityAfterRemoteOwnersAndKeepsStatusCapability(t *testing.T) {
	base := strings.TrimSpace(os.Getenv("PRIVACY_CONSUMER_PG_DSN"))
	if base == "" {
		t.Skip("PRIVACY_CONSUMER_PG_DSN is not set")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "identity_privacy_" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", "")
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) }()
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration, err := privacy.Schema(privacy.CurrentSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, migration.UpSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
CREATE TABLE identities(id uuid PRIMARY KEY, email text NOT NULL, status text NOT NULL);
CREATE TABLE audit_logs(actor_identity_id uuid, target_identity_id uuid);
CREATE TABLE oidc_oauth_requests(subject text);
CREATE TABLE oidc_refresh_tokens(subject text);
CREATE TABLE github_identity_bindings(
  id uuid PRIMARY KEY, identity_id uuid NOT NULL, provider_account_id text NOT NULL,
  provider_node_id text NOT NULL, login_snapshot text NOT NULL,
  avatar_url_snapshot text NOT NULL, status text NOT NULL,
  unbound_at timestamptz, erased_at timestamptz, updated_at timestamptz NOT NULL
);
CREATE TABLE identity_privacy_requests(
  request_id text PRIMARY KEY, identity_id uuid NOT NULL,
  status_token_hash text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO identities(id,email,status)
VALUES ('00000000-0000-0000-0000-000000000101','owner@example.com','active');
INSERT INTO audit_logs(target_identity_id)
VALUES ('00000000-0000-0000-0000-000000000101');
INSERT INTO github_identity_bindings(
  id, identity_id, provider_account_id, provider_node_id, login_snapshot,
  avatar_url_snapshot, status, updated_at
) VALUES (
  '00000000-0000-0000-0000-000000000201',
  '00000000-0000-0000-0000-000000000101',
  '123456', 'U_123456', 'publisher-login', 'https://images.test/a',
  'active', now()
);
`); err != nil {
		t.Fatal(err)
	}
	blogOwner := privacycatalog.Blog()
	catalog := privacy.MustCompile(Definition(blogOwner))
	remote := map[privacy.OwnerKey]privacy.OwnerHost{}
	for _, owner := range catalog.Owners() {
		if owner.Ref.Key == privacycatalog.IdentityOwner {
			continue
		}
		definition := owner
		host, err := privacy.NewMemoryOwnerHost(definition, privacy.MemoryOwnerHostOptions{
			Executor: privacy.OwnerExecutorFunc(func(ctx context.Context, instruction privacy.OwnerInstruction) (privacy.OwnerOutcome, error) {
				var accountExists bool
				if err := db.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM identities WHERE id='00000000-0000-0000-0000-000000000101')`).
					Scan(&accountExists); err != nil {
					return privacy.OwnerOutcome{}, err
				}
				if !accountExists {
					t.Fatalf("%s owner ran after identity finalization", definition.Ref.Key)
				}
				results := make([]privacy.DatasetOutcome, 0, len(instruction.Command.Datasets))
				for _, dataset := range instruction.Command.Datasets {
					results = append(results, privacy.DatasetOutcome{
						Dataset: dataset, Disposition: privacy.DispositionNotFound,
					})
				}
				return privacy.OwnerOutcome{Terminal: true, Results: results}, nil
			}),
		})
		if err != nil {
			t.Fatal(err)
		}
		remote[owner.Ref.Key] = host
	}
	workCatalog := work.MustCompile(WorkDefinition())
	workModule, err := work.NewMemory(workCatalog, work.MemoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPostgres(ctx, Options{
		DB: db, InstanceKey: "identity:test", Remote: remote,
		Owners: []privacy.OwnerDefinition{blogOwner}, Work: workModule,
	})
	if err != nil {
		t.Fatal(err)
	}
	requestedAt := time.Now().UTC().Truncate(time.Microsecond)
	view, err := service.OpenErasure(
		ctx, "00000000-0000-0000-0000-000000000101", "owner@example.com",
		"identity-erasure-command-1", strings.Repeat("s", 48), requestedAt,
		privacy.VerificationEvidence{
			VerifiedAt: requestedAt, Method: "active_identity_session",
			Assurance: "single_factor", VerificationRef: "session-digest",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != privacy.RequestComplete || view.Summary.Retained != 1 {
		t.Fatalf("view = %#v", view)
	}
	jobs, err := workModule.List(ctx, work.ListQuery{Kinds: []work.Kind{DriveKind}})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("privacy work jobs = %#v, %v", jobs, err)
	}
	if _, err := service.WorkHandler().Handle(ctx, jobs[0], nil); err != nil {
		t.Fatalf("completed request work handler = %v", err)
	}
	var accountExists bool
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM identities WHERE id='00000000-0000-0000-0000-000000000101')`).
		Scan(&accountExists); err != nil {
		t.Fatal(err)
	}
	if accountExists {
		t.Fatal("identity account was not finalized")
	}
	var (
		accountID     string
		login         string
		bindingStatus string
		erasedAt      *time.Time
	)
	if err := db.QueryRowContext(ctx, `
SELECT provider_account_id, login_snapshot, status, erased_at
FROM github_identity_bindings
WHERE id='00000000-0000-0000-0000-000000000201'
`).Scan(&accountID, &login, &bindingStatus, &erasedAt); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(accountID, "erased:") || login != "erased" ||
		bindingStatus != "unbound" || erasedAt == nil {
		t.Fatalf(
			"GitHub binding was not privacy-scrubbed: account=%q login=%q status=%q erased=%v",
			accountID, login, bindingStatus, erasedAt,
		)
	}
	status, err := service.GetByToken(ctx, strings.Repeat("s", 48), view.ID)
	if err != nil || status.Phase != privacy.RequestComplete {
		t.Fatalf("status after account deletion = %#v, %v", status, err)
	}
	if _, err := service.GetByToken(ctx, strings.Repeat("x", 48), view.ID); err == nil {
		t.Fatal("wrong status capability accepted")
	}
}
