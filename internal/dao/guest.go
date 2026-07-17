package dao

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

func (p *PG) CreateGuestSession(ctx context.Context, session model.GuestSession) error {
	_, err := p.db.Model("guest_sessions").Ctx(ctx).Data(gdb.Map{
		"id":         session.ID,
		"token_hash": session.TokenHash,
		"client_id":  session.ClientID,
		"created_at": session.CreatedAt,
		"last_seen":  session.LastSeen,
		"expires_at": session.ExpiresAt,
	}).Insert()
	return err
}

func (p *PG) GetGuestSession(ctx context.Context, tokenHash string) (model.GuestSession, error) {
	var session model.GuestSession
	if err := p.db.Model("guest_sessions").Ctx(ctx).Where("token_hash", tokenHash).Scan(&session); err != nil {
		return model.GuestSession{}, err
	}
	if session.ID == "" {
		return model.GuestSession{}, repo.ErrGuestSessionMissing
	}
	return session, nil
}

func (p *PG) ClaimGuestSession(ctx context.Context, tokenHash, identityID string, claimedAt time.Time) (model.GuestSession, error) {
	var session model.GuestSession
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		result, err := tx.Ctx(ctx).Exec(`UPDATE guest_sessions
SET claimed_identity_id = COALESCE(claimed_identity_id, ?::uuid),
    claimed_at = COALESCE(claimed_at, ?)
WHERE token_hash = ?
  AND (claimed_identity_id IS NULL OR claimed_identity_id = ?::uuid)`, identityID, claimedAt, tokenHash, identityID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			var existing model.GuestSession
			if err := tx.Model("guest_sessions").Where("token_hash", tokenHash).Scan(&existing); err != nil {
				return err
			}
			if existing.ID == "" {
				return repo.ErrGuestSessionMissing
			}
			return repo.ErrGuestClaimConflict
		}
		return tx.Model("guest_sessions").Where("token_hash", tokenHash).Scan(&session)
	})
	if errors.Is(err, repo.ErrGuestSessionMissing) || errors.Is(err, repo.ErrGuestClaimConflict) {
		return model.GuestSession{}, err
	}
	return session, err
}

var _ repo.GuestSessionStore = (*PG)(nil)
