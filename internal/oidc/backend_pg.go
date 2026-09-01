package oidc

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
)

// txKey marks a gdb transaction stored in the context by BeginTX.
type txKey struct{}

// pgBackend implements Backend against PostgreSQL (gdb), with real transactions
// (fosite wraps the authorize->token and refresh-rotation chains in BeginTX/Commit).
type pgBackend struct {
	db gdb.DB
}

// NewPGBackend builds a PG-backed Backend.
func NewPGBackend(db gdb.DB) Backend { return &pgBackend{db: db} }

// model returns the gdb model bound to the ambient tx (if any) for a table.
func (b *pgBackend) model(ctx context.Context, table string) *gdb.Model {
	if tx, ok := ctx.Value(txKey{}).(gdb.TX); ok && tx != nil {
		return tx.Model(table).Ctx(ctx)
	}
	return b.db.Model(table).Ctx(ctx)
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// ── generic ──────────────────────────────────────────────────────────────────

func (b *pgBackend) PutGeneric(ctx context.Context, kind, sig string, r Record) error {
	_, err := b.model(ctx, "oidc_oauth_requests").OnConflict("kind", "signature").Data(gdb.Map{
		"kind": kind, "signature": sig, "request_id": r.RequestID,
		"client_id": r.ClientID, "subject": r.Subject, "active": r.Active,
		"expires_at": nullableTime(r.ExpiresAt), "data": string(r.Data),
	}).Save()
	return err
}

func (b *pgBackend) GetGeneric(ctx context.Context, kind, sig string) (Record, error) {
	row, err := b.model(ctx, "oidc_oauth_requests").
		Where("kind", kind).Where("signature", sig).One()
	if err != nil {
		return Record{}, err
	}
	if row.IsEmpty() {
		return Record{}, ErrBackendNotFound
	}
	return Record{
		RequestID: row["request_id"].String(),
		ClientID:  row["client_id"].String(),
		Subject:   row["subject"].String(),
		Active:    row["active"].Bool(),
		Data:      row["data"].Bytes(),
	}, nil
}

func (b *pgBackend) DeactivateGeneric(ctx context.Context, kind, sig string) error {
	_, err := b.model(ctx, "oidc_oauth_requests").
		Where("kind", kind).Where("signature", sig).Data("active", false).Update()
	return err
}

func (b *pgBackend) DeleteGeneric(ctx context.Context, kind, sig string) error {
	_, err := b.model(ctx, "oidc_oauth_requests").
		Where("kind", kind).Where("signature", sig).Delete()
	return err
}

// ── refresh ──────────────────────────────────────────────────────────────────

func (b *pgBackend) PutRefresh(ctx context.Context, sig string, r RefreshRecord) error {
	_, err := b.model(ctx, "oidc_refresh_tokens").OnConflict("signature").Data(gdb.Map{
		"signature": sig, "request_id": r.RequestID, "client_id": r.ClientID,
		"subject": r.Subject, "session_id": r.SessionID, "access_signature": r.AccessSignature,
		"active": r.Active, "expires_at": nullableTime(r.ExpiresAt), "data": string(r.Data),
	}).Save()
	return err
}

func (b *pgBackend) GetRefresh(ctx context.Context, sig string) (RefreshRecord, error) {
	row, err := b.model(ctx, "oidc_refresh_tokens").Where("signature", sig).One()
	if err != nil {
		return RefreshRecord{}, err
	}
	if row.IsEmpty() {
		return RefreshRecord{}, ErrBackendNotFound
	}
	rec := RefreshRecord{
		RequestID: row["request_id"].String(),
		ClientID:  row["client_id"].String(),
		Subject:   row["subject"].String(),
		SessionID: row["session_id"].String(),
		Active:    row["active"].Bool(),
		Data:      row["data"].Bytes(),
	}
	if !rec.Active {
		return rec, ErrBackendInactive
	}
	return rec, nil
}

func (b *pgBackend) DeactivateRefresh(ctx context.Context, sig string) error {
	_, err := b.model(ctx, "oidc_refresh_tokens").Where("signature", sig).Data("active", false).Update()
	return err
}

func (b *pgBackend) DeleteRefresh(ctx context.Context, sig string) error {
	_, err := b.model(ctx, "oidc_refresh_tokens").Where("signature", sig).Delete()
	return err
}

func (b *pgBackend) RevokeRefreshByRequestID(ctx context.Context, requestID string) error {
	_, err := b.model(ctx, "oidc_refresh_tokens").Where("request_id", requestID).Delete()
	return err
}

func (b *pgBackend) RevokeRefreshBySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	_, err := b.model(ctx, "oidc_refresh_tokens").Where("session_id", sessionID).Delete()
	return err
}

func (b *pgBackend) RevokeRefreshBySubject(ctx context.Context, subject string) error {
	_, err := b.model(ctx, "oidc_refresh_tokens").Where("subject", subject).Delete()
	return err
}

func (b *pgBackend) PutRefreshReplay(ctx context.Context, receipt RefreshReplayReceipt) error {
	_, _ = b.model(ctx, "oidc_refresh_replay_receipts").Where("expires_at <= ?", time.Now().UTC()).Delete()
	_, err := b.model(ctx, "oidc_refresh_replay_receipts").OnConflict("key_digest").Data(gdb.Map{
		"key_digest": receipt.KeyDigest, "client_id": receipt.ClientID, "request_id": receipt.RequestID,
		"response_ciphertext": receipt.ResponseCiphertext, "expires_at": receipt.ExpiresAt,
	}).Save()
	return err
}
func (b *pgBackend) GetRefreshReplay(ctx context.Context, key, client string, now time.Time) (RefreshReplayReceipt, bool, error) {
	row, err := b.db.GetOne(ctx, `SELECT receipts.key_digest, receipts.client_id, receipts.request_id, receipts.response_ciphertext, receipts.expires_at
		FROM oidc_refresh_replay_receipts AS receipts
		WHERE receipts.key_digest=$1 AND receipts.client_id=$2 AND receipts.expires_at>$3
		AND EXISTS (SELECT 1 FROM oidc_refresh_tokens AS tokens WHERE tokens.request_id=receipts.request_id AND tokens.active=TRUE AND (tokens.expires_at IS NULL OR tokens.expires_at>$3))`, key, client, now)
	if err != nil {
		return RefreshReplayReceipt{}, false, err
	}
	if row.IsEmpty() {
		return RefreshReplayReceipt{}, false, nil
	}
	return RefreshReplayReceipt{KeyDigest: key, ClientID: client, RequestID: row["request_id"].String(), ResponseCiphertext: row["response_ciphertext"].Bytes(), ExpiresAt: row["expires_at"].Time()}, true, nil
}

// ── Transactional ─────────────────────────────────────────────────────────────

func (b *pgBackend) BeginTX(ctx context.Context) (context.Context, error) {
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, txKey{}, tx), nil
}

func (b *pgBackend) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(txKey{}).(gdb.TX)
	if !ok || tx == nil {
		return errors.New("oidc pgBackend: commit without active tx")
	}
	return tx.Commit()
}

func (b *pgBackend) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(txKey{}).(gdb.TX)
	if !ok || tx == nil {
		return errors.New("oidc pgBackend: rollback without active tx")
	}
	return tx.Rollback()
}

var _ Backend = (*pgBackend)(nil)
