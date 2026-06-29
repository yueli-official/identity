package dao

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

const sessionsTable = "identity_sessions"

func (p *PG) CreateSession(ctx context.Context, s model.Session, ttl time.Duration) error {
	if ttl > 0 && s.ExpiresAt.IsZero() {
		base := s.CreatedAt
		if base.IsZero() {
			base = time.Now().UTC()
		}
		s.ExpiresAt = base.Add(ttl)
	}
	_, err := p.db.Model(sessionsTable).Ctx(ctx).Data(g.Map{
		"id":          s.ID,
		"identity_id": s.IdentityID,
		"created_at":  s.CreatedAt,
		"last_seen":   s.LastSeen,
		"user_agent":  s.UserAgent,
		"ip":          s.IP,
		"device":      s.Device,
		"expires_at":  s.ExpiresAt,
	}).OnConflict("id").Save()
	return err
}

func (p *PG) GetSession(ctx context.Context, id string) (model.Session, error) {
	var out model.Session
	if err := p.db.Model(sessionsTable).Ctx(ctx).
		Where("id", id).
		Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC()).
		Scan(&out); err != nil {
		return model.Session{}, err
	}
	if out.ID == "" {
		return model.Session{}, repo.ErrSessionNotFound
	}
	return out, nil
}

func (p *PG) DeleteSession(ctx context.Context, id string) error {
	_, err := p.db.Model(sessionsTable).Ctx(ctx).Where("id", id).Delete()
	return err
}

func (p *PG) ListSessionsByIdentity(ctx context.Context, identityID string) ([]model.Session, error) {
	var out []model.Session
	if err := p.db.Model(sessionsTable).Ctx(ctx).
		Where("identity_id", identityID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC()).
		OrderDesc("created_at").
		Scan(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *PG) DeleteSessionsByIdentity(ctx context.Context, identityID string) error {
	_, err := p.db.Model(sessionsTable).Ctx(ctx).Where("identity_id", identityID).Delete()
	return err
}
