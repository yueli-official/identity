package dao

import (
	"context"
	"time"

	"platform/services/identity/internal/repo"
)

// InsertPAT is a stub pending Task 4 (PG PAT dao implementation).
func (p *PG) InsertPAT(_ context.Context, _ repo.PATRow) (int64, error) {
	panic("dao.PG.InsertPAT: not implemented (Task 4)")
}

// ListPATByIdentity is a stub pending Task 4.
func (p *PG) ListPATByIdentity(_ context.Context, _ string) ([]repo.PATRow, error) {
	panic("dao.PG.ListPATByIdentity: not implemented (Task 4)")
}

// GetPATByHash is a stub pending Task 4.
func (p *PG) GetPATByHash(_ context.Context, _ string) (repo.PATRow, bool, error) {
	panic("dao.PG.GetPATByHash: not implemented (Task 4)")
}

// DeletePAT is a stub pending Task 4.
func (p *PG) DeletePAT(_ context.Context, _ int64, _ string) (bool, error) {
	panic("dao.PG.DeletePAT: not implemented (Task 4)")
}

// TouchPATLastUsed is a stub pending Task 4.
func (p *PG) TouchPATLastUsed(_ context.Context, _ int64, _ time.Time) error {
	panic("dao.PG.TouchPATLastUsed: not implemented (Task 4)")
}

// CountPATByIdentity is a stub pending Task 4.
func (p *PG) CountPATByIdentity(_ context.Context, _ string) (int, error) {
	panic("dao.PG.CountPATByIdentity: not implemented (Task 4)")
}
