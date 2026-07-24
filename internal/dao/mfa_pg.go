package dao

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/authentication"
)

func (p *PG) CountActiveTOTP(ctx context.Context, identityID string) (int, error) {
	return p.db.Model("totp_authenticators").Ctx(ctx).
		Where("identity_id", identityID).
		Where("status", "active").
		Count()
}

func (p *PG) CreatePendingTOTP(
	ctx context.Context,
	authenticator authentication.TOTPAuthenticator,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		identity, err := tx.GetValue(`
SELECT identities.id
FROM identities
JOIN identity_sessions
  ON identity_sessions.identity_id = identities.id
 AND identity_sessions.id = ?
 AND identity_sessions.expires_at > NOW()
WHERE identities.id = ? AND identities.status = 'active'
FOR UPDATE OF identities
`, authenticator.BindingSessionID, authenticator.IdentityID)
		if err != nil {
			return err
		}
		if identity.IsNil() || identity.String() == "" {
			return authentication.ErrTOTPEnrollmentInvalid
		}

		if _, err := tx.Exec(`
UPDATE totp_authenticators
SET status = 'revoked',
    binding_session_id = NULL,
    enrollment_expires_at = NULL,
    revoked_at = ?,
    revoked_reason = 'superseded enrollment',
    updated_at = ?
WHERE identity_id = ? AND status = 'pending'
`, authenticator.CreatedAt, authenticator.CreatedAt, authenticator.IdentityID); err != nil {
			return err
		}

		_, err = tx.Model("totp_authenticators").Ctx(ctx).
			Data(totpAuthenticatorData(authenticator)).
			Insert()
		return err
	})
}

func (p *PG) GetTOTP(
	ctx context.Context,
	identityID, authenticatorID string,
) (authentication.TOTPAuthenticator, error) {
	var row totpAuthenticatorRow
	if err := p.db.Model("totp_authenticators").Ctx(ctx).
		Where("id", authenticatorID).
		Where("identity_id", identityID).
		WhereNot("status", "revoked").
		Scan(&row); err != nil {
		return authentication.TOTPAuthenticator{}, err
	}
	if row.ID == "" {
		return authentication.TOTPAuthenticator{}, authentication.ErrTOTPNotFound
	}
	return row.authenticator(), nil
}

func (p *PG) RecordTOTPFailure(ctx context.Context, authenticatorID string, max int) error {
	_, err := p.db.Exec(ctx, `
UPDATE totp_authenticators
SET failed_attempts = LEAST(failed_attempts + 1, ?),
    updated_at = NOW()
WHERE id = ? AND status = 'pending'
`, max, authenticatorID)
	return err
}

func (p *PG) ActivateTOTP(
	ctx context.Context,
	authenticator authentication.TOTPAuthenticator,
	sessionID string,
	lastUsedStep int64,
	recoverySetID string,
	recoveryCodes []authentication.RecoveryCode,
	now time.Time,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		recoverySession, err := tx.GetValue(`
SELECT authentication_events.recovery
FROM identities
JOIN identity_sessions
  ON identity_sessions.identity_id = identities.id
 AND identity_sessions.id = ?
 AND identity_sessions.expires_at > NOW()
JOIN authentication_events
  ON authentication_events.id = identity_sessions.authentication_event_id
WHERE identities.id = ? AND identities.status = 'active'
FOR UPDATE OF identities
`, sessionID, authenticator.IdentityID)
		if err != nil {
			return err
		}
		if recoverySession.IsNil() {
			return authentication.ErrTOTPEnrollmentInvalid
		}

		activated, err := tx.GetValue(`
UPDATE totp_authenticators
SET status = 'active',
    binding_session_id = NULL,
    enrollment_expires_at = NULL,
    failed_attempts = 0,
    last_used_step = ?,
    verified_at = ?,
    updated_at = ?
WHERE id = ?
  AND identity_id = ?
  AND status = 'pending'
  AND binding_session_id = ?
  AND enrollment_expires_at > ?
  AND failed_attempts < 5
RETURNING id
`,
			lastUsedStep, now, now, authenticator.ID, authenticator.IdentityID,
			sessionID, now,
		)
		if err != nil {
			return err
		}
		if activated.IsNil() || activated.String() == "" {
			return authentication.ErrTOTPEnrollmentInvalid
		}
		if recoverySession.Bool() {
			if _, err := tx.Exec(`
UPDATE totp_authenticators
SET status = 'revoked',
    revoked_at = ?,
    revoked_reason = 'replaced during account recovery',
    updated_at = ?
WHERE identity_id = ? AND id <> ? AND status IN ('active', 'suspended')
`, now, now, authenticator.IdentityID, authenticator.ID); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(`
INSERT INTO authentication_policies(
    identity_id, second_factor_required, policy_version, updated_at
)
VALUES (?, TRUE, 1, ?)
ON CONFLICT(identity_id) DO UPDATE
SET second_factor_required = TRUE,
    policy_version = authentication_policies.policy_version + 1,
    updated_at = EXCLUDED.updated_at
`, authenticator.IdentityID, now); err != nil {
			return err
		}

		if _, err := tx.Exec(`
UPDATE recovery_code_sets
SET status = 'revoked',
    revoked_at = ?,
    revoked_reason = 'regenerated'
WHERE identity_id = ? AND status = 'active'
`, now, authenticator.IdentityID); err != nil {
			return err
		}
		if _, err := tx.Model("recovery_code_sets").Ctx(ctx).Data(g.Map{
			"id": recoverySetID, "identity_id": authenticator.IdentityID,
			"status": "active", "generated_at": now,
		}).Insert(); err != nil {
			return err
		}
		for _, code := range recoveryCodes {
			if _, err := tx.Model("recovery_codes").Ctx(ctx).Data(g.Map{
				"id": code.ID, "set_id": recoverySetID,
				"code_digest": code.Digest, "created_at": now,
			}).Insert(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *PG) ListTOTP(
	ctx context.Context,
	identityID string,
) ([]authentication.TOTPAuthenticator, error) {
	var rows []totpAuthenticatorRow
	if err := p.db.Model("totp_authenticators").Ctx(ctx).
		Where("identity_id", identityID).
		WhereIn("status", []string{"active", "suspended"}).
		OrderDesc("created_at").
		Scan(&rows); err != nil {
		return nil, err
	}
	out := make([]authentication.TOTPAuthenticator, len(rows))
	for index := range rows {
		out[index] = rows[index].authenticator()
	}
	return out, nil
}

func (p *PG) RevokeTOTP(
	ctx context.Context,
	identityID, authenticatorID, reason string,
	now time.Time,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		identity, err := tx.GetValue(`
SELECT id
FROM identities
WHERE id = ? AND status = 'active'
FOR UPDATE
`, identityID)
		if err != nil {
			return err
		}
		if identity.IsNil() || identity.String() == "" {
			return authentication.ErrTOTPNotFound
		}

		revoked, err := tx.GetValue(`
UPDATE totp_authenticators
SET status = 'revoked',
    binding_session_id = NULL,
    enrollment_expires_at = NULL,
    revoked_at = ?,
    revoked_reason = ?,
    updated_at = ?
WHERE id = ?
  AND identity_id = ?
  AND status IN ('active', 'suspended')
RETURNING id
`, now, reason, now, authenticatorID, identityID)
		if err != nil {
			return err
		}
		if revoked.IsNil() || revoked.String() == "" {
			return authentication.ErrTOTPNotFound
		}

		active, err := tx.GetValue(`
SELECT COUNT(*)
FROM totp_authenticators
WHERE identity_id = ? AND status = 'active'
`, identityID)
		if err != nil {
			return err
		}
		if active.Int() > 0 {
			return nil
		}

		if _, err := tx.Exec(`
INSERT INTO authentication_policies(
    identity_id, second_factor_required, policy_version, updated_at
)
VALUES (?, FALSE, 1, ?)
ON CONFLICT(identity_id) DO UPDATE
SET second_factor_required = FALSE,
    policy_version = authentication_policies.policy_version + 1,
    updated_at = EXCLUDED.updated_at
`, identityID, now); err != nil {
			return err
		}
		_, err = tx.Exec(`
UPDATE recovery_code_sets
SET status = 'revoked',
    revoked_at = ?,
    revoked_reason = 'last second factor removed'
WHERE identity_id = ? AND status = 'active'
`, now, identityID)
		return err
	})
}

func (p *PG) IsSecondFactorRequired(
	ctx context.Context,
	identityID string,
) (bool, error) {
	value, err := p.db.GetValue(ctx, `
SELECT COALESCE((
    SELECT second_factor_required
    FROM authentication_policies
    WHERE identity_id = ?
), FALSE)
`, identityID)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

func (p *PG) CreateAuthenticationTransaction(
	ctx context.Context,
	transaction authentication.AuthenticationTransaction,
) error {
	_, err := p.db.Model("authentication_transactions").Ctx(ctx).Data(g.Map{
		"id": transaction.ID, "kind": transaction.Kind,
		"identity_id": transaction.IdentityID,
		"session_id":  nilIfEmpty(transaction.SessionID),
		"audience":    transaction.Audience, "action": transaction.Action,
		"resource_digest": nilIfBytesEmpty(transaction.ResourceDigest),
		"requirement":     string(transaction.Requirement), "state": string(transaction.State),
		"expires_at": transaction.ExpiresAt, "created_at": transaction.CreatedAt,
	}).Insert()
	return err
}

func (p *PG) GetAuthenticationTransaction(
	ctx context.Context,
	id string,
) (authentication.AuthenticationTransaction, error) {
	var row authenticationTransactionRow
	if err := p.db.Model("authentication_transactions").Ctx(ctx).
		Where("id", id).Scan(&row); err != nil {
		return authentication.AuthenticationTransaction{}, err
	}
	if row.ID == "" {
		return authentication.AuthenticationTransaction{},
			authentication.ErrAuthenticationTransactionInvalid
	}
	return row.transaction(), nil
}

func (p *PG) RecordAuthenticationTransactionFailure(
	ctx context.Context,
	id string,
	max int,
) error {
	_, err := p.db.Exec(ctx, `
UPDATE authentication_transactions
SET failed_attempts = LEAST(failed_attempts + 1, ?)
WHERE id = ? AND consumed_at IS NULL AND expires_at > NOW()
`, max, id)
	return err
}

func (p *PG) CompleteTOTPLogin(
	ctx context.Context,
	transaction authentication.AuthenticationTransaction,
	authenticatorID string,
	lastUsedStep int64,
	session authentication.Session,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		identity, err := tx.GetValue(`
SELECT id
FROM identities
WHERE id = ? AND status = 'active'
FOR UPDATE
`, transaction.IdentityID)
		if err != nil {
			return err
		}
		if identity.IsNil() || identity.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		consumed, err := tx.GetValue(`
UPDATE authentication_transactions
SET consumed_at = ?
WHERE id = ?
  AND kind = 'mfa_login'
  AND identity_id = ?
  AND consumed_at IS NULL
  AND expires_at > ?
  AND failed_attempts < 5
RETURNING id
`,
			session.Authentication.AuthenticatedAt, transaction.ID,
			transaction.IdentityID, session.Authentication.AuthenticatedAt,
		)
		if err != nil {
			return err
		}
		if consumed.IsNil() || consumed.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		updated, err := tx.GetValue(`
UPDATE totp_authenticators
SET last_used_step = ?,
    last_used_at = ?,
    updated_at = ?
WHERE id = ?
  AND identity_id = ?
  AND status = 'active'
  AND (last_used_step IS NULL OR last_used_step < ?)
RETURNING id
`,
			lastUsedStep, session.Authentication.AuthenticatedAt,
			session.Authentication.AuthenticatedAt, authenticatorID,
			transaction.IdentityID, lastUsedStep,
		)
		if err != nil {
			return err
		}
		if updated.IsNil() || updated.String() == "" {
			return authentication.ErrTOTPCodeInvalid
		}
		return insertAuthenticationSessionTX(ctx, tx, session)
	})
}

func (p *PG) CompleteRecoveryLogin(
	ctx context.Context,
	transaction authentication.AuthenticationTransaction,
	codeDigest []byte,
	session authentication.Session,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		identity, err := tx.GetValue(`
SELECT id
FROM identities
WHERE id = ? AND status = 'active'
FOR UPDATE
`, transaction.IdentityID)
		if err != nil {
			return err
		}
		if identity.IsNil() || identity.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		consumed, err := tx.GetValue(`
UPDATE authentication_transactions
SET consumed_at = ?
WHERE id = ?
  AND kind = 'mfa_login'
  AND identity_id = ?
  AND consumed_at IS NULL
  AND expires_at > ?
  AND failed_attempts < 5
RETURNING id
`,
			session.Authentication.AuthenticatedAt, transaction.ID,
			transaction.IdentityID, session.Authentication.AuthenticatedAt,
		)
		if err != nil {
			return err
		}
		if consumed.IsNil() || consumed.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		setID, err := tx.GetValue(`
UPDATE recovery_codes AS codes
SET consumed_at = ?
FROM recovery_code_sets AS sets
WHERE codes.set_id = sets.id
  AND sets.identity_id = ?
  AND sets.status = 'active'
  AND codes.code_digest = ?
  AND codes.consumed_at IS NULL
RETURNING codes.set_id
`, session.Authentication.AuthenticatedAt, transaction.IdentityID, codeDigest)
		if err != nil {
			return err
		}
		if setID.IsNil() || setID.String() == "" {
			return authentication.ErrRecoveryCodeInvalid
		}
		remaining, err := tx.GetValue(`
SELECT COUNT(*)
FROM recovery_codes
WHERE set_id = ? AND consumed_at IS NULL
`, setID.String())
		if err != nil {
			return err
		}
		if remaining.Int() == 0 {
			if _, err := tx.Exec(`
UPDATE recovery_code_sets
SET status = 'exhausted', exhausted_at = ?
WHERE id = ? AND status = 'active'
`, session.Authentication.AuthenticatedAt, setID.String()); err != nil {
				return err
			}
		}
		return insertAuthenticationSessionTX(ctx, tx, session)
	})
}

func (p *PG) CompleteTOTPTransaction(
	ctx context.Context,
	transaction authentication.AuthenticationTransaction,
	authenticatorID string,
	lastUsedStep int64,
	now time.Time,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		session, err := tx.GetValue(`
SELECT sessions.id
FROM identity_sessions AS sessions
JOIN identities ON identities.id = sessions.identity_id
WHERE sessions.id = ?
  AND sessions.identity_id = ?
  AND sessions.expires_at > ?
  AND identities.status = 'active'
FOR SHARE OF sessions
`, transaction.SessionID, transaction.IdentityID, now)
		if err != nil {
			return err
		}
		if session.IsNil() || session.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		consumed, err := tx.GetValue(`
UPDATE authentication_transactions
SET consumed_at = ?
WHERE id = ?
  AND kind = 'step_up'
  AND identity_id = ?
  AND session_id = ?
  AND consumed_at IS NULL
  AND expires_at > ?
  AND failed_attempts < 5
RETURNING id
`, now, transaction.ID, transaction.IdentityID, transaction.SessionID, now)
		if err != nil {
			return err
		}
		if consumed.IsNil() || consumed.String() == "" {
			return authentication.ErrAuthenticationTransactionInvalid
		}
		updated, err := tx.GetValue(`
UPDATE totp_authenticators
SET last_used_step = ?, last_used_at = ?, updated_at = ?
WHERE id = ?
  AND identity_id = ?
  AND status = 'active'
  AND (last_used_step IS NULL OR last_used_step < ?)
RETURNING id
`, lastUsedStep, now, now, authenticatorID, transaction.IdentityID, lastUsedStep)
		if err != nil {
			return err
		}
		if updated.IsNil() || updated.String() == "" {
			return authentication.ErrTOTPCodeInvalid
		}
		return nil
	})
}

func totpAuthenticatorData(value authentication.TOTPAuthenticator) g.Map {
	return g.Map{
		"id": value.ID, "identity_id": value.IdentityID, "label": value.Label,
		"secret_ciphertext": value.SecretCiphertext, "key_version": value.KeyVersion,
		"algorithm": value.Algorithm, "digits": value.Digits,
		"period_seconds": value.PeriodSeconds, "status": value.Status,
		"binding_session_id":    nilIfEmpty(value.BindingSessionID),
		"enrollment_expires_at": value.EnrollmentExpiresAt,
		"failed_attempts":       value.FailedAttempts, "last_used_step": value.LastUsedStep,
		"created_at": value.CreatedAt, "verified_at": value.VerifiedAt,
		"updated_at": value.UpdatedAt, "last_used_at": value.LastUsedAt,
	}
}

type totpAuthenticatorRow struct {
	ID                  string
	IdentityID          string
	Label               string
	SecretCiphertext    []byte
	KeyVersion          int
	Algorithm           string
	Digits              int
	PeriodSeconds       int
	Status              string
	BindingSessionID    string
	EnrollmentExpiresAt *time.Time
	FailedAttempts      int
	LastUsedStep        *int64
	CreatedAt           time.Time
	VerifiedAt          *time.Time
	UpdatedAt           time.Time
	LastUsedAt          *time.Time
}

type authenticationTransactionRow struct {
	ID             string
	Kind           string
	IdentityID     string
	SessionID      string
	Audience       string
	Action         string
	ResourceDigest []byte
	Requirement    json.RawMessage
	State          json.RawMessage
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	FailedAttempts int
	CreatedAt      time.Time
}

func (row authenticationTransactionRow) transaction() authentication.AuthenticationTransaction {
	return authentication.AuthenticationTransaction{
		ID: row.ID, Kind: row.Kind, IdentityID: row.IdentityID,
		SessionID: row.SessionID, Audience: row.Audience, Action: row.Action,
		ResourceDigest: row.ResourceDigest, Requirement: row.Requirement, State: row.State,
		ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt,
		FailedAttempts: row.FailedAttempts, CreatedAt: row.CreatedAt,
	}
}

func nilIfBytesEmpty(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func (row totpAuthenticatorRow) authenticator() authentication.TOTPAuthenticator {
	return authentication.TOTPAuthenticator{
		ID: row.ID, IdentityID: row.IdentityID, Label: row.Label,
		SecretCiphertext: row.SecretCiphertext, KeyVersion: row.KeyVersion,
		Algorithm: row.Algorithm, Digits: row.Digits, PeriodSeconds: row.PeriodSeconds,
		Status: row.Status, BindingSessionID: row.BindingSessionID,
		EnrollmentExpiresAt: row.EnrollmentExpiresAt, FailedAttempts: row.FailedAttempts,
		LastUsedStep: row.LastUsedStep, CreatedAt: row.CreatedAt,
		VerifiedAt: row.VerifiedAt, UpdatedAt: row.UpdatedAt, LastUsedAt: row.LastUsedAt,
	}
}

var _ authentication.MFAStore = (*PG)(nil)
