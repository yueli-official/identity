package dao

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/identity/internal/githubbinding"
)

type githubBindingRow struct {
	ID                string     `orm:"id"`
	IdentityID        string     `orm:"identity_id"`
	Provider          string     `orm:"provider"`
	ProviderAccountID string     `orm:"provider_account_id"`
	ProviderNodeID    string     `orm:"provider_node_id"`
	Login             string     `orm:"login_snapshot"`
	AvatarURL         string     `orm:"avatar_url_snapshot"`
	Status            string     `orm:"status"`
	VerifiedAt        time.Time  `orm:"verified_at"`
	LastVerifiedAt    time.Time  `orm:"last_verified_at"`
	UnboundAt         *time.Time `orm:"unbound_at"`
	BlockedAt         *time.Time `orm:"blocked_at"`
	CreatedAt         time.Time  `orm:"created_at"`
	UpdatedAt         time.Time  `orm:"updated_at"`
}

func (p *PG) CreateAttempt(
	ctx context.Context,
	attempt githubbinding.Attempt,
) error {
	_, err := p.db.Model("github_binding_attempts").Ctx(ctx).Data(g.Map{
		"id": attempt.ID, "state_digest": attempt.StateDigest,
		"identity_id": attempt.IdentityID, "session_digest": attempt.SessionDigest,
		"verifier_ciphertext": attempt.VerifierCiphertext,
		"return_to":           attempt.ReturnTo,
		"expires_at":          attempt.ExpiresAt,
		"created_at":          attempt.CreatedAt,
	}).Insert()
	if isUniqueViolation(err) {
		return githubbinding.ErrInvalidAttempt
	}
	return err
}

func (p *PG) ConsumeAttempt(
	ctx context.Context,
	stateDigest string,
	sessionDigest string,
	now time.Time,
) (githubbinding.Attempt, error) {
	row, err := p.db.GetOne(ctx, `
UPDATE github_binding_attempts
SET consumed_at = $3
WHERE state_digest = $1
  AND session_digest = $2
  AND consumed_at IS NULL
  AND expires_at > $3
RETURNING id, state_digest, identity_id, session_digest, verifier_ciphertext,
          return_to, expires_at, consumed_at, created_at
`, stateDigest, sessionDigest, now)
	if err != nil {
		return githubbinding.Attempt{}, err
	}
	if len(row) == 0 {
		return githubbinding.Attempt{}, githubbinding.ErrInvalidAttempt
	}
	consumedAt := row["consumed_at"].Time()
	return githubbinding.Attempt{
		ID: row["id"].String(), StateDigest: row["state_digest"].String(),
		IdentityID:         row["identity_id"].String(),
		SessionDigest:      row["session_digest"].String(),
		VerifierCiphertext: row["verifier_ciphertext"].String(),
		ReturnTo:           row["return_to"].String(), ExpiresAt: row["expires_at"].Time(),
		ConsumedAt: &consumedAt, CreatedAt: row["created_at"].Time(),
	}, nil
}

func (p *PG) Bind(
	ctx context.Context,
	identityID string,
	account githubbinding.Account,
	now time.Time,
) (githubbinding.BindResult, error) {
	if existing, found, err := p.activeGitHubBinding(ctx, account.AccountID); err != nil {
		return githubbinding.BindResult{}, err
	} else if found {
		return p.refreshGitHubBinding(ctx, existing, identityID, account, now)
	}

	id := identifier.MustNew().String()
	_, err := p.db.Exec(ctx, `
INSERT INTO github_identity_bindings (
    id, identity_id, provider, provider_account_id, provider_node_id,
    login_snapshot, avatar_url_snapshot, status, verified_at,
    last_verified_at, created_at, updated_at
) VALUES ($1, $2, 'github', $3, $4, $5, $6, 'active', $7, $7, $7, $7)
`, id, identityID, account.AccountID, account.NodeID, account.Login, account.AvatarURL, now)
	if err != nil {
		if isUniqueViolation(err) {
			existing, found, loadErr := p.activeGitHubBinding(ctx, account.AccountID)
			if loadErr != nil {
				return githubbinding.BindResult{}, loadErr
			}
			if found {
				return p.refreshGitHubBinding(ctx, existing, identityID, account, now)
			}
		}
		return githubbinding.BindResult{}, err
	}
	return githubbinding.BindResult{
		Binding: githubbinding.Binding{
			ID: id, IdentityID: identityID, Provider: githubbinding.ProviderGitHub,
			ProviderAccountID: account.AccountID, ProviderNodeID: account.NodeID,
			Login: account.Login, AvatarURL: account.AvatarURL,
			Status: githubbinding.StatusActive, VerifiedAt: now,
			LastVerifiedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		Created: true,
	}, nil
}

func (p *PG) refreshGitHubBinding(
	ctx context.Context,
	existing githubbinding.Binding,
	identityID string,
	account githubbinding.Account,
	now time.Time,
) (githubbinding.BindResult, error) {
	if existing.IdentityID != identityID {
		return githubbinding.BindResult{}, githubbinding.ErrBindingConflict
	}
	renamed := existing.Login != account.Login
	row, err := p.db.GetOne(ctx, `
UPDATE github_identity_bindings
SET provider_node_id = $2, login_snapshot = $3, avatar_url_snapshot = $4,
    last_verified_at = $5, updated_at = $5
WHERE id = $1 AND status = 'active'
RETURNING *
`, existing.ID, account.NodeID, account.Login, account.AvatarURL, now)
	if err != nil {
		return githubbinding.BindResult{}, err
	}
	if len(row) == 0 {
		return githubbinding.BindResult{}, githubbinding.ErrBindingConflict
	}
	return githubbinding.BindResult{
		Binding: githubBindingFromRecord(row), Renamed: renamed,
	}, nil
}

func (p *PG) ListByIdentity(
	ctx context.Context,
	identityID string,
) ([]githubbinding.Binding, error) {
	var rows []githubBindingRow
	if err := p.db.Model("github_identity_bindings").Ctx(ctx).
		Where("identity_id", identityID).OrderDesc("created_at").Scan(&rows); err != nil {
		return nil, err
	}
	result := make([]githubbinding.Binding, len(rows))
	for index := range rows {
		result[index] = githubBindingFromRow(rows[index])
	}
	return result, nil
}

func (p *PG) FindActiveByAccount(
	ctx context.Context,
	accountID string,
) (githubbinding.Binding, error) {
	binding, found, err := p.activeGitHubBinding(ctx, accountID)
	if err != nil {
		return githubbinding.Binding{}, err
	}
	if !found {
		return githubbinding.Binding{}, githubbinding.ErrBindingInactive
	}
	return binding, nil
}

func (p *PG) activeGitHubBinding(
	ctx context.Context,
	accountID string,
) (githubbinding.Binding, bool, error) {
	row, err := p.db.GetOne(ctx, `
SELECT *
FROM github_identity_bindings
WHERE provider = 'github' AND provider_account_id = $1 AND status = 'active'
`, accountID)
	if err != nil {
		return githubbinding.Binding{}, false, err
	}
	if len(row) == 0 {
		return githubbinding.Binding{}, false, nil
	}
	return githubBindingFromRecord(row), true, nil
}

func (p *PG) Unbind(
	ctx context.Context,
	identityID string,
	bindingID string,
	now time.Time,
) (githubbinding.Binding, error) {
	row, err := p.db.GetOne(ctx, `
UPDATE github_identity_bindings
SET status = 'unbound', unbound_at = $3, updated_at = $3
WHERE id = $1 AND identity_id = $2 AND status = 'active'
RETURNING *
`, bindingID, identityID, now)
	if err != nil {
		return githubbinding.Binding{}, err
	}
	if len(row) == 0 {
		return githubbinding.Binding{}, githubbinding.ErrBindingNotFound
	}
	return githubBindingFromRecord(row), nil
}

func (p *PG) BlockByAccount(
	ctx context.Context,
	accountID string,
	login string,
	now time.Time,
) ([]githubbinding.Binding, error) {
	rows, err := p.db.GetAll(ctx, `
UPDATE github_identity_bindings
SET status = 'blocked', login_snapshot = $2, blocked_at = $3, updated_at = $3
WHERE provider = 'github' AND provider_account_id = $1 AND status = 'active'
RETURNING *
`, accountID, login, now)
	if err != nil {
		return nil, err
	}
	result := make([]githubbinding.Binding, len(rows))
	for index := range rows {
		result[index] = githubBindingFromRecord(rows[index])
	}
	return result, nil
}

func githubBindingFromRow(row githubBindingRow) githubbinding.Binding {
	return githubbinding.Binding{
		ID: row.ID, IdentityID: row.IdentityID, Provider: row.Provider,
		ProviderAccountID: row.ProviderAccountID, ProviderNodeID: row.ProviderNodeID,
		Login: row.Login, AvatarURL: row.AvatarURL, Status: row.Status,
		VerifiedAt: row.VerifiedAt, LastVerifiedAt: row.LastVerifiedAt,
		UnboundAt: row.UnboundAt, BlockedAt: row.BlockedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func githubBindingFromRecord(row gdb.Record) githubbinding.Binding {
	binding := githubbinding.Binding{
		ID: row["id"].String(), IdentityID: row["identity_id"].String(),
		Provider:          row["provider"].String(),
		ProviderAccountID: row["provider_account_id"].String(),
		ProviderNodeID:    row["provider_node_id"].String(),
		Login:             row["login_snapshot"].String(),
		AvatarURL:         row["avatar_url_snapshot"].String(),
		Status:            row["status"].String(), VerifiedAt: row["verified_at"].Time(),
		LastVerifiedAt: row["last_verified_at"].Time(),
		CreatedAt:      row["created_at"].Time(), UpdatedAt: row["updated_at"].Time(),
	}
	if !row["unbound_at"].IsNil() {
		value := row["unbound_at"].Time()
		binding.UnboundAt = &value
	}
	if !row["blocked_at"].IsNil() {
		value := row["blocked_at"].Time()
		binding.BlockedAt = &value
	}
	return binding
}

var _ githubbinding.Store = (*PG)(nil)
