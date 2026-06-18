package dao

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"platform/services/identity/internal/repo"
)

// patRow is a DAO-internal struct that mirrors the pat_tokens table columns.
// It uses orm tags so that gdb.Scan can map snake_case column names to Go fields.
// Nullable timestamps are *gtime.Time so gdb maps NULL correctly.
// Scopes is read as a raw JSON string from Postgres; we unmarshal it ourselves.
type patRow struct {
	ID          int64       `orm:"id"`
	IdentityID  string      `orm:"identity_id"`
	Name        string      `orm:"name"`
	TokenHash   string      `orm:"token_hash"`
	TokenPrefix string      `orm:"token_prefix"`
	Scopes      string      `orm:"scopes"`
	ExpiresAt   *gtime.Time `orm:"expires_at"`
	LastUsedAt  *gtime.Time `orm:"last_used_at"`
	CreatedAt   gtime.Time  `orm:"created_at"`
}

// toRepoRow converts a DAO-internal patRow to a repo.PATRow.
// Nil scopes in the JSON (or empty/missing JSON) normalize to []string{}.
func (r patRow) toRepoRow() (repo.PATRow, error) {
	scopes := []string{}
	if r.Scopes != "" && r.Scopes != "null" {
		if err := json.Unmarshal([]byte(r.Scopes), &scopes); err != nil {
			return repo.PATRow{}, err
		}
	}
	if scopes == nil {
		scopes = []string{}
	}

	out := repo.PATRow{
		ID:          r.ID,
		IdentityID:  r.IdentityID,
		Name:        r.Name,
		TokenHash:   r.TokenHash,
		TokenPrefix: r.TokenPrefix,
		Scopes:      scopes,
		CreatedAt:   r.CreatedAt.Time,
	}
	if r.ExpiresAt != nil {
		t := r.ExpiresAt.Time
		out.ExpiresAt = &t
	}
	if r.LastUsedAt != nil {
		t := r.LastUsedAt.Time
		out.LastUsedAt = &t
	}
	return out, nil
}

// InsertPAT creates a new PAT row in pat_tokens and returns the assigned id.
// Scopes nil/empty normalizes to JSON "[]". ExpiresAt and LastUsedAt nil → SQL NULL.
// CreatedAt zero → omitted so the DB default (now()) fills it.
func (p *PG) InsertPAT(ctx context.Context, row repo.PATRow) (int64, error) {
	scopes := row.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return 0, err
	}

	data := g.Map{
		"identity_id":  row.IdentityID,
		"name":         row.Name,
		"token_hash":   row.TokenHash,
		"token_prefix": row.TokenPrefix,
		"scopes":       string(scopesJSON),
		"expires_at":   nilIfTimeNil(row.ExpiresAt),
		"last_used_at": nilIfTimeNil(row.LastUsedAt),
	}
	// Only include created_at when the caller provides a non-zero value;
	// otherwise let the DB default (now()) fill it — same pattern as pg_audit.go
	// which omits occurred_at from the insert map.
	if !row.CreatedAt.IsZero() {
		data["created_at"] = row.CreatedAt
	}

	id, err := p.db.Model("pat_tokens").Ctx(ctx).Data(data).InsertAndGetId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListPATByIdentity returns all PATs for identityID, newest-first (id DESC).
// Nil scopes normalize to []string{} for parity with the memory implementation.
func (p *PG) ListPATByIdentity(ctx context.Context, identityID string) ([]repo.PATRow, error) {
	var rows []patRow
	if err := p.db.Model("pat_tokens").Ctx(ctx).
		Where("identity_id", identityID).
		OrderDesc("id").
		Scan(&rows); err != nil {
		return nil, err
	}

	out := make([]repo.PATRow, 0, len(rows))
	for _, r := range rows {
		pr, err := r.toRepoRow()
		if err != nil {
			return nil, err
		}
		out = append(out, pr)
	}
	return out, nil
}

// GetPATByHash looks up a PAT by its HMAC-SHA256 token hash.
// Returns (row, true, nil) when found; (zero, false, nil) when not found.
// Nil scopes normalize to []string{}.
func (p *PG) GetPATByHash(ctx context.Context, hash string) (repo.PATRow, bool, error) {
	var row patRow
	if err := p.db.Model("pat_tokens").Ctx(ctx).
		Where("token_hash", hash).
		Scan(&row); err != nil {
		return repo.PATRow{}, false, err
	}
	// gdb.Scan leaves the struct at zero value when no rows are found.
	if row.ID == 0 {
		return repo.PATRow{}, false, nil
	}
	pr, err := row.toRepoRow()
	if err != nil {
		return repo.PATRow{}, false, err
	}
	return pr, true, nil
}

// DeletePAT deletes the token only when it exists AND belongs to identityID.
// Returns (true, nil) when a row was deleted; (false, nil) otherwise (not found
// or wrong owner — consistent with the memory implementation).
func (p *PG) DeletePAT(ctx context.Context, id int64, identityID string) (bool, error) {
	result, err := p.db.Model("pat_tokens").Ctx(ctx).
		Where("id", id).
		Where("identity_id", identityID).
		Delete()
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// TouchPATLastUsed sets last_used_at to t on the PAT with the given id.
// It is a no-op (no error) when no row matches the id.
func (p *PG) TouchPATLastUsed(ctx context.Context, id int64, t time.Time) error {
	_, err := p.db.Model("pat_tokens").Ctx(ctx).
		Where("id", id).
		Data(g.Map{"last_used_at": t}).
		Update()
	return err
}

// CountPATByIdentity returns the number of PAT rows owned by identityID.
func (p *PG) CountPATByIdentity(ctx context.Context, identityID string) (int, error) {
	n, err := p.db.Model("pat_tokens").Ctx(ctx).
		Where("identity_id", identityID).
		Count()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// nilIfTimeNil converts a *time.Time to nil (SQL NULL) when the pointer is nil,
// or to the underlying time.Time value when non-nil. Used to map optional
// timestamp fields to nullable TIMESTAMPTZ columns.
func nilIfTimeNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// Compile-time interface assertion: PG must satisfy repo.PATRepo.
var _ repo.PATRepo = (*PG)(nil)
