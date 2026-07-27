// Package identityprivacy implements the instance-local data-rights
// coordinator and Identity's own finalizing Owner.
package identityprivacy

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yueli-official/foundation/go/privacy"
	"github.com/yueli-official/foundation/go/work"

	identitycatalog "github.com/yueli-official/identity/internal/identityprivacy/catalog"
)

func Definition(configuredOwners ...privacy.OwnerDefinition) privacy.Definition {
	owner := identitycatalog.Identity()
	return privacy.Definition{
		Version: privacy.DefinitionVersion, Consumer: "platform.identity",
		SubjectKinds:   identitycatalog.SubjectKinds(),
		DataCategories: identitycatalog.Categories(),
		RetentionRules: identitycatalog.RetentionRules(),
		Limits:         privacy.Limits{MaxAliases: 64},
		Owner:          &owner,
		Coordination: &privacy.CoordinationDefinition{
			Owners: identitycatalog.Owners(configuredOwners...), RightsPolicies: identitycatalog.RightsPolicies(),
		},
	}
}

type Service struct {
	db          *sql.DB
	coordinator privacy.Coordinator
	host        privacy.OwnerHost
	work        work.Enqueuer
	owners      []privacy.OwnerDefinition
}

type Options struct {
	DB          *sql.DB
	InstanceKey string
	Remote      map[privacy.OwnerKey]privacy.OwnerHost
	Owners      []privacy.OwnerDefinition
	Work        work.Enqueuer
}

func NewPostgres(ctx context.Context, options Options) (*Service, error) {
	catalog, err := privacy.Compile(Definition(options.Owners...))
	if err != nil {
		return nil, err
	}
	host, err := privacy.NewPostgresOwnerHost(
		ctx, catalog, privacy.PostgresOptions{DB: options.DB, InstanceKey: options.InstanceKey},
		privacy.OwnerExecutorFunc((&executor{db: options.DB}).execute),
	)
	if err != nil {
		return nil, err
	}
	router := privacy.OwnerRouterFunc(func(_ context.Context, owner privacy.OwnerKey) (privacy.OwnerHost, error) {
		if owner == identitycatalog.IdentityOwner {
			return host, nil
		}
		if remote := options.Remote[owner]; remote != nil {
			return remote, nil
		}
		return nil, &privacy.Error{
			Kind: privacy.ErrorOwnerUnavailable, Field: "owner",
			Message: fmt.Sprintf("%s owner is not configured", owner), Retryable: true,
		}
	})
	coordinator, err := privacy.NewPostgresCoordinator(
		ctx, catalog, privacy.PostgresOptions{DB: options.DB, InstanceKey: options.InstanceKey}, router,
	)
	if err != nil {
		return nil, err
	}
	return &Service{
		db: options.DB, coordinator: coordinator, host: host, work: options.Work,
		owners: catalog.Owners(),
	}, nil
}

func (service *Service) OwnerHost() privacy.OwnerHost { return service.host }

func (service *Service) OpenErasure(
	ctx context.Context, userID, email string, commandKey privacy.IdempotencyKey,
	statusToken string, requestedAt time.Time, verification privacy.VerificationEvidence,
) (privacy.RightsRequestView, error) {
	current := privacy.SubjectRef{
		Owner: identitycatalog.IdentityOwner, Kind: identitycatalog.UserSubject, Value: userID,
	}
	aliases := make([]privacy.SubjectRef, 0, len(service.owners)*2)
	for _, owner := range service.owners {
		if owner.Ref.Key == identitycatalog.IdentityOwner {
			continue
		}
		for _, kind := range owner.SubjectKinds {
			switch kind {
			case identitycatalog.UserSubject:
				aliases = append(aliases, privacy.SubjectRef{Owner: owner.Ref.Key, Kind: kind, Value: userID})
			case identitycatalog.SubscriberSubject:
				aliases = append(aliases, privacy.SubjectRef{Owner: owner.Ref.Key, Kind: kind, Value: email})
			}
		}
	}
	view, err := service.coordinator.Open(ctx, privacy.OpenRightsRequest{
		IdempotencyKey: commandKey,
		Subject: privacy.SubjectContext{
			Current: &current,
			Aliases: aliases,
		},
		Operation: privacy.RightErasure, Verification: verification,
		RequestedAt: requestedAt.UTC(), Channel: "account_center",
	})
	if err != nil {
		return privacy.RightsRequestView{}, err
	}
	result, err := service.db.ExecContext(ctx, `
INSERT INTO identity_privacy_requests(request_id, identity_id, status_token_hash, created_at)
VALUES ($1,$2::uuid,$3,now())
ON CONFLICT (request_id) DO UPDATE
SET status_token_hash=EXCLUDED.status_token_hash
WHERE identity_privacy_requests.identity_id=EXCLUDED.identity_id
  AND identity_privacy_requests.status_token_hash=EXCLUDED.status_token_hash`,
		view.ID, userID, tokenDigest(statusToken))
	if err != nil {
		return privacy.RightsRequestView{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return privacy.RightsRequestView{}, &privacy.Error{
			Kind: privacy.ErrorConflict, Field: "status_token",
			Message: "does not match the existing request",
		}
	}
	if err := service.enqueue(ctx, view.ID); err != nil {
		return privacy.RightsRequestView{}, err
	}
	driven, err := service.coordinator.Drive(ctx, privacy.DriveRightsRequest{
		Request: view.ID, Budget: privacy.DriveBudget{MaxOwnerAttempts: 8, MaxDuration: 5 * time.Second},
	})
	if err != nil {
		return privacy.RightsRequestView{}, err
	}
	return driven.View, nil
}

func (service *Service) GetByToken(
	ctx context.Context, statusToken string, requestID privacy.RightsRequestID,
) (privacy.RightsRequestView, error) {
	var allowed bool
	err := service.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM identity_privacy_requests
  WHERE request_id=$1 AND status_token_hash=$2
)`, requestID, tokenDigest(statusToken)).Scan(&allowed)
	if err != nil {
		return privacy.RightsRequestView{}, err
	}
	if !allowed {
		return privacy.RightsRequestView{}, &privacy.Error{
			Kind: privacy.ErrorNotFound, Field: "request", Message: "is not found",
		}
	}
	if err := service.enqueue(ctx, requestID); err != nil {
		return privacy.RightsRequestView{}, err
	}
	driven, err := service.coordinator.Drive(ctx, privacy.DriveRightsRequest{
		Request: requestID, Budget: privacy.DriveBudget{MaxOwnerAttempts: 8, MaxDuration: 5 * time.Second},
	})
	if err != nil {
		return privacy.RightsRequestView{}, err
	}
	return driven.View, nil
}

const DriveKind work.Kind = "privacy.rights.drive"

func WorkDefinition() work.Definition {
	return work.Definition{
		Version: work.DefinitionVersion,
		Queues:  []work.QueueDefinition{{Key: "privacy", Concurrency: 4}},
		Kinds: []work.KindDefinition{{
			Key: DriveKind, Queue: "privacy",
			DefaultAttempts: 200, MaxAttempts: 500, Timeout: 30 * time.Second,
		}},
		Retry: work.RetryPolicy{
			BaseDelay: time.Minute, MaxDelay: 6 * time.Hour, Jitter: 0.2,
		},
	}
}

func (service *Service) enqueue(ctx context.Context, requestID privacy.RightsRequestID) error {
	if service.work == nil {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{"requestId": string(requestID)})
	_, err := service.work.Enqueue(ctx, work.Request{
		Kind: DriveKind, Payload: payload,
		IdempotencyKey: "privacy.rights.drive:" + string(requestID),
	})
	return err
}

func (service *Service) WorkHandler() work.Handler {
	return work.HandlerFunc(func(ctx context.Context, job work.Job, _ work.Progress) (work.Result, error) {
		var payload struct {
			RequestID string `json:"requestId"`
		}
		if err := json.Unmarshal(job.Payload, &payload); err != nil || strings.TrimSpace(payload.RequestID) == "" {
			return work.Result{}, work.Permanent(errors.New("privacy rights work payload is invalid"))
		}
		driven, err := service.coordinator.Drive(ctx, privacy.DriveRightsRequest{
			Request: privacy.RightsRequestID(payload.RequestID),
			Budget:  privacy.DriveBudget{MaxOwnerAttempts: 8, MaxDuration: 20 * time.Second},
		})
		if err != nil {
			return work.Result{}, err
		}
		if driven.View.Phase != privacy.RequestComplete {
			return work.Result{}, errors.New("privacy rights request still has pending owners")
		}
		return work.Result{Summary: string(driven.View.Phase)}, nil
	})
}

func tokenDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type executor struct{ db *sql.DB }

func (executor *executor) execute(ctx context.Context, instruction privacy.OwnerInstruction) (privacy.OwnerOutcome, error) {
	if instruction.Command.Operation != privacy.RightErasure {
		return privacy.OwnerOutcome{}, fmt.Errorf("identity privacy: unsupported operation %q", instruction.Command.Operation)
	}
	userIDs := identitySubjects(instruction.Command.Subject)
	tx, err := executor.db.BeginTx(ctx, nil)
	if err != nil {
		return privacy.OwnerOutcome{}, err
	}
	defer func() { _ = tx.Rollback() }()
	results := make([]privacy.DatasetOutcome, 0, len(instruction.Command.Datasets))
	for _, dataset := range instruction.Command.Datasets {
		switch dataset {
		case identitycatalog.IdentityAccountDataset:
			var count int64
			for _, userID := range userIDs {
				// Fosite token rows intentionally have no FK to identities.
				for _, query := range []string{
					`DELETE FROM oidc_oauth_requests WHERE subject=$1`,
					`DELETE FROM oidc_refresh_tokens WHERE subject=$1`,
				} {
					if _, err := tx.ExecContext(ctx, query, userID); err != nil {
						return privacy.OwnerOutcome{}, err
					}
				}
				// Publisher subjects and already-issued proofs remain stable, but
				// the external GitHub identifier/profile projection is personal
				// data. Scrub it before deleting Identity; the history row keeps
				// only a non-resolvable binding ID and lifecycle timestamps.
				if _, err := tx.ExecContext(ctx, `
UPDATE github_identity_bindings
SET status = 'unbound',
    provider_account_id = 'erased:' || id::text,
    provider_node_id = '',
    login_snapshot = 'erased',
    avatar_url_snapshot = '',
    unbound_at = COALESCE(unbound_at, now()),
    erased_at = now(),
    updated_at = now()
WHERE identity_id = $1::uuid
  AND erased_at IS NULL
`, userID); err != nil {
					return privacy.OwnerOutcome{}, err
				}
				result, err := tx.ExecContext(ctx, `DELETE FROM identities WHERE id=$1::uuid`, userID)
				if err != nil {
					return privacy.OwnerOutcome{}, err
				}
				n, _ := result.RowsAffected()
				count += n
			}
			disposition := privacy.DispositionDeleted
			if count == 0 {
				disposition = privacy.DispositionNotFound
			}
			results = append(results, privacy.DatasetOutcome{
				Dataset: dataset, Disposition: disposition, Count: count,
			})
		case identitycatalog.IdentityAuditDataset:
			var count int64
			for _, userID := range userIDs {
				var current int64
				if err := tx.QueryRowContext(ctx, `
SELECT count(*) FROM audit_logs
WHERE actor_identity_id=$1::uuid OR target_identity_id=$1::uuid`, userID).Scan(&current); err != nil {
					return privacy.OwnerOutcome{}, err
				}
				count += current
			}
			disposition := privacy.DispositionRetained
			reason := privacy.ReasonCode("security_audit_retention")
			if count == 0 {
				disposition, reason = privacy.DispositionNotFound, ""
			}
			results = append(results, privacy.DatasetOutcome{
				Dataset: dataset, Disposition: disposition, Count: count, Reason: reason,
			})
		default:
			return privacy.OwnerOutcome{}, fmt.Errorf("identity privacy: unknown dataset %q", dataset)
		}
	}
	if err := tx.Commit(); err != nil {
		return privacy.OwnerOutcome{}, err
	}
	return privacy.OwnerOutcome{Terminal: true, Results: results}, nil
}

func identitySubjects(subjects privacy.SubjectContext) []string {
	all := append([]privacy.SubjectRef(nil), subjects.Aliases...)
	if subjects.Current != nil {
		all = append(all, *subjects.Current)
	}
	result := make([]string, 0, len(all))
	for _, subject := range all {
		if subject.Owner == identitycatalog.IdentityOwner &&
			subject.Kind == identitycatalog.UserSubject && strings.TrimSpace(subject.Value) != "" {
			result = append(result, subject.Value)
		}
	}
	return result
}
