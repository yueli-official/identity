package dao

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/repo"
)

// auditRow is a DAO-internal struct that mirrors the audit_logs table columns.
// It uses orm tags so that gdb.Scan can map snake_case column names to Go fields
// whose names don't follow the default auto-conversion rules (e.g.
// actor_identity_id → ActorID would mis-map without an explicit tag).
type auditRow struct {
	ID         int64  `orm:"id"`
	Event      string `orm:"event"`
	ActorID    string `orm:"actor_identity_id"`
	TargetID   string `orm:"target_identity_id"`
	ActorEmail string `orm:"actor_email"`
	IP         string `orm:"ip"`
	UserAgent  string `orm:"user_agent"`
	ClientID   string `orm:"client_id"`
	RequestID  string `orm:"request_id"`
	Result     string `orm:"result"`
	// Detail is read as a raw JSON string from Postgres; we unmarshal it ourselves.
	Detail     string    `orm:"detail"`
	OccurredAt time.Time `orm:"occurred_at"`
}

// InsertAudit writes a single audit event row to audit_logs.
// UUID columns (actor_identity_id, target_identity_id) must be NULL, not "",
// when the caller passes an empty string — Postgres rejects "" for uuid columns.
// The remaining nullable TEXT columns are also written as NULL when empty,
// for cleanliness (NULL preferred over empty string in nullable columns).
func (p *PG) InsertAudit(ctx context.Context, row repo.AuditRow) error {
	detail := row.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}

	_, err = p.db.Model("audit_logs").Ctx(ctx).Data(g.Map{
		"event":              row.Event,
		"actor_identity_id":  nilIfEmpty(row.ActorID),
		"target_identity_id": nilIfEmpty(row.TargetID),
		"actor_email":        nilIfEmpty(row.ActorEmail),
		"ip":                 nilIfEmpty(row.IP),
		"user_agent":         nilIfEmpty(row.UserAgent),
		"client_id":          nilIfEmpty(row.ClientID),
		"request_id":         nilIfEmpty(row.RequestID),
		"result":             orDefault(row.Result, "success"),
		"detail":             string(detailJSON),
	}).Insert()
	return err
}

// QueryAudit returns audit_logs rows matching the filter, newest-first (ORDER BY id DESC).
// IdentityID matches actor_identity_id OR target_identity_id.
// Limit is clamped: 0 → 50, negative → 50, >200 → 200.
// Offset is clamped: negative → 0.
func (p *PG) QueryAudit(ctx context.Context, f repo.AuditFilter) ([]repo.AuditRow, error) {
	limit := f.Limit
	switch {
	case limit <= 0:
		limit = 50
	case limit > 200:
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	q := p.db.Model("audit_logs").Ctx(ctx).
		Fields("id, event, actor_identity_id, target_identity_id, actor_email, ip, user_agent, client_id, request_id, result, detail, occurred_at").
		Order("id DESC").
		Limit(limit).
		Offset(offset)

	if f.IdentityID != "" {
		q = q.Where("actor_identity_id = ? OR target_identity_id = ?", f.IdentityID, f.IdentityID)
	}
	if f.Event != "" {
		q = q.Where("event", f.Event)
	}

	var rows []auditRow
	if err := q.Scan(&rows); err != nil {
		return nil, err
	}

	out := make([]repo.AuditRow, 0, len(rows))
	for _, r := range rows {
		var detail map[string]any
		// The "null" guard handles rows written outside this DAO: our insert path
		// normalizes nil → {} so it never emits literal JSON null, but a row from
		// another writer might store the string "null".
		if r.Detail != "" && r.Detail != "null" {
			if err := json.Unmarshal([]byte(r.Detail), &detail); err != nil {
				return nil, err
			}
		}
		if detail == nil {
			detail = map[string]any{}
		}
		out = append(out, repo.AuditRow{
			ID:         r.ID,
			Event:      r.Event,
			ActorID:    r.ActorID,
			TargetID:   r.TargetID,
			ActorEmail: r.ActorEmail,
			IP:         r.IP,
			UserAgent:  r.UserAgent,
			ClientID:   r.ClientID,
			RequestID:  r.RequestID,
			Result:     r.Result,
			Detail:     detail,
			OccurredAt: r.OccurredAt,
		})
	}
	return out, nil
}

// Compile-time interface assertion.
var _ repo.AuditRepo = (*PG)(nil)
