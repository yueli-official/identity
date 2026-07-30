package dao

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/yueli-official/identity/internal/authentication"
)

func (p *PG) GetOrCreatePasskeyUser(
	ctx context.Context,
	identityID string,
	candidate []byte,
) ([]byte, error) {
	value, err := p.db.GetValue(ctx, `
INSERT INTO webauthn_users(identity_id, user_handle)
VALUES (?, ?)
ON CONFLICT(identity_id) DO UPDATE SET identity_id = EXCLUDED.identity_id
RETURNING user_handle
`, identityID, candidate)
	if err != nil {
		return nil, err
	}
	return value.Bytes(), nil
}

func (p *PG) GetPasskeyUserByIdentity(
	ctx context.Context,
	identityID string,
) (authentication.PasskeyUser, error) {
	var row struct {
		IdentityID string
		UserHandle []byte
	}
	if err := p.db.Model("webauthn_users AS wu").Ctx(ctx).
		Fields("wu.identity_id", "wu.user_handle").
		InnerJoin("identities AS i", "i.id = wu.identity_id").
		Where("wu.identity_id", identityID).
		Where("i.status", "active").
		Scan(&row); err != nil {
		return authentication.PasskeyUser{}, err
	}
	if row.IdentityID == "" {
		return authentication.PasskeyUser{}, authentication.ErrPasskeyNotFound
	}
	credentials, err := p.ListPasskeys(ctx, identityID)
	if err != nil {
		return authentication.PasskeyUser{}, err
	}
	return authentication.PasskeyUser{
		IdentityID: row.IdentityID, UserHandle: row.UserHandle, Credentials: credentials,
	}, nil
}

func (p *PG) GetPasskeyUserByHandle(
	ctx context.Context,
	handle []byte,
) (authentication.PasskeyUser, error) {
	var row struct {
		IdentityID string
		UserHandle []byte
	}
	if err := p.db.Model("webauthn_users AS wu").Ctx(ctx).
		Fields("wu.identity_id", "wu.user_handle").
		InnerJoin("identities AS i", "i.id = wu.identity_id").
		Where("wu.user_handle", handle).
		Where("i.status", "active").
		Scan(&row); err != nil {
		return authentication.PasskeyUser{}, err
	}
	if row.IdentityID == "" {
		return authentication.PasskeyUser{}, authentication.ErrPasskeyNotFound
	}
	credentials, err := p.ListPasskeys(ctx, row.IdentityID)
	if err != nil {
		return authentication.PasskeyUser{}, err
	}
	return authentication.PasskeyUser{
		IdentityID: row.IdentityID, UserHandle: row.UserHandle, Credentials: credentials,
	}, nil
}

func (p *PG) ListPasskeys(
	ctx context.Context,
	identityID string,
) ([]authentication.PasskeyCredential, error) {
	var rows []passkeyRow
	if err := p.db.Model("webauthn_credentials").Ctx(ctx).
		Where("identity_id", identityID).
		WhereNot("status", "revoked").
		OrderDesc("created_at").
		Scan(&rows); err != nil {
		return nil, err
	}
	out := make([]authentication.PasskeyCredential, len(rows))
	for i := range rows {
		out[i] = rows[i].credential()
	}
	return out, nil
}

func (p *PG) CountActivePasskeys(ctx context.Context, identityID string) (int, error) {
	return p.db.Model("webauthn_credentials").Ctx(ctx).
		Where("identity_id", identityID).
		Where("status", "active").
		Count()
}

func (p *PG) RenamePasskey(
	ctx context.Context,
	identityID, credentialID, label string,
	now time.Time,
) (authentication.PasskeyCredential, error) {
	var row passkeyRow
	err := p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		value, err := tx.GetValue(`
UPDATE webauthn_credentials
SET label = ?, updated_at = ?, version = version + 1
WHERE id = ? AND identity_id = ? AND status <> 'revoked'
RETURNING id
`, label, now, credentialID, identityID)
		if err != nil {
			return err
		}
		if value.IsNil() || value.String() == "" {
			return authentication.ErrPasskeyNotFound
		}
		return tx.Model("webauthn_credentials").Ctx(ctx).
			Where("id", credentialID).Scan(&row)
	})
	if err != nil {
		return authentication.PasskeyCredential{}, err
	}
	if row.ID == "" {
		return authentication.PasskeyCredential{}, authentication.ErrPasskeyNotFound
	}
	return row.credential(), nil
}

func (p *PG) RevokePasskey(
	ctx context.Context,
	identityID, credentialID, reason string,
	now time.Time,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		locked, err := tx.GetValue(
			`SELECT id FROM identities WHERE id = ? AND status = 'active' FOR UPDATE`,
			identityID,
		)
		if err != nil {
			return err
		}
		if locked.IsNil() || locked.String() == "" {
			return authentication.ErrPasskeyNotFound
		}

		target, err := tx.GetValue(`
SELECT id
FROM webauthn_credentials
WHERE id = ? AND identity_id = ? AND status <> 'revoked'
`, credentialID, identityID)
		if err != nil {
			return err
		}
		if target.IsNil() || target.String() == "" {
			return authentication.ErrPasskeyNotFound
		}

		alternative, err := tx.GetValue(`
SELECT
    EXISTS (
        SELECT 1 FROM credentials_password WHERE identity_id = ?
    )
    OR EXISTS (
        SELECT 1 FROM credentials_oauth WHERE identity_id = ?
    )
    OR EXISTS (
        SELECT 1
        FROM webauthn_credentials
        WHERE identity_id = ? AND id <> ? AND status = 'active'
    )
`, identityID, identityID, identityID, credentialID)
		if err != nil {
			return err
		}
		if !alternative.Bool() {
			return authentication.ErrLastAuthenticator
		}

		value, err := tx.GetValue(`
UPDATE webauthn_credentials
SET status = 'revoked',
    revoked_at = ?,
    revoked_reason = ?,
    updated_at = ?,
    version = version + 1
WHERE id = ? AND identity_id = ? AND status <> 'revoked'
RETURNING id
`, now, reason, now, credentialID, identityID)
		if err != nil {
			return err
		}
		if value.IsNil() || value.String() == "" {
			return authentication.ErrPasskeyNotFound
		}
		return nil
	})
}

func (p *PG) CreateCeremony(ctx context.Context, ceremony authentication.Ceremony) error {
	_, err := p.db.Model("authentication_ceremonies").Ctx(ctx).Data(g.Map{
		"id": ceremony.ID, "kind": ceremony.Kind,
		"identity_id":      nilIfEmpty(ceremony.IdentityID),
		"session_id":       nilIfEmpty(ceremony.SessionID),
		"challenge_digest": ceremony.ChallengeDigest,
		"library_state":    string(ceremony.LibraryState),
		"expires_at":       ceremony.ExpiresAt, "created_at": ceremony.CreatedAt,
	}).Insert()
	return err
}

func (p *PG) GetCeremony(ctx context.Context, id string) (authentication.Ceremony, error) {
	var row ceremonyRow
	if err := p.db.Model("authentication_ceremonies").Ctx(ctx).
		Where("id", id).Scan(&row); err != nil {
		return authentication.Ceremony{}, err
	}
	if row.ID == "" {
		return authentication.Ceremony{}, authentication.ErrCeremonyInvalid
	}
	return row.ceremony(), nil
}

func (p *PG) RecordCeremonyFailure(ctx context.Context, id string, max int) error {
	_, err := p.db.Exec(ctx, `
UPDATE authentication_ceremonies
SET failed_attempts = LEAST(failed_attempts + 1, ?)
WHERE id = ? AND consumed_at IS NULL
`, max, id)
	return err
}

func (p *PG) CompletePasskeyRegistration(
	ctx context.Context,
	ceremony authentication.Ceremony,
	credential authentication.PasskeyCredential,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		session, err := tx.GetValue(`
SELECT sessions.id
FROM identity_sessions AS sessions
JOIN identities
  ON identities.id = sessions.identity_id
 AND identities.status = 'active'
WHERE sessions.id = ?
  AND sessions.identity_id = ?
  AND sessions.expires_at > NOW()
FOR SHARE OF sessions
`, ceremony.SessionID, ceremony.IdentityID)
		if err != nil {
			return err
		}
		if session.IsNil() || session.String() == "" {
			return authentication.ErrCeremonyInvalid
		}
		if err := consumeCeremonyTX(ctx, tx, ceremony, ceremony.IdentityID, ceremony.SessionID); err != nil {
			return err
		}
		_, err = tx.Model("webauthn_credentials").Ctx(ctx).Data(passkeyData(credential)).Insert()
		if isUniqueViolation(err) {
			return authentication.ErrPasskeyExists
		}
		return err
	})
}

func (p *PG) CompletePasskeyAuthentication(
	ctx context.Context,
	ceremony authentication.Ceremony,
	credential authentication.PasskeyCredential,
	session authentication.Session,
	_ time.Duration,
) error {
	return p.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		tx = tx.Ctx(ctx)
		identity, err := tx.GetValue(`
SELECT id
FROM identities
WHERE id = ? AND status = 'active'
FOR UPDATE
`, session.IdentityID)
		if err != nil {
			return err
		}
		if identity.IsNil() || identity.String() == "" {
			return authentication.ErrPasskeyNotFound
		}
		if err := consumeCeremonyTX(ctx, tx, ceremony, "", ""); err != nil {
			return err
		}
		value, err := tx.GetValue(`
UPDATE webauthn_credentials
SET sign_count = GREATEST(sign_count, ?),
    clone_warning = clone_warning OR ?,
    flags = ?,
    backup_state = ?,
    last_used_at = ?,
    updated_at = ?,
    version = version + 1
WHERE id = ?
  AND identity_id = ?
  AND status = 'active'
  AND version = ?
  AND backup_eligible = ?
RETURNING id
`,
			credential.SignCount, credential.CloneWarning, credential.Flags,
			credential.BackupState, session.Authentication.AuthenticatedAt,
			session.Authentication.AuthenticatedAt, credential.ID, session.IdentityID,
			credential.Version, credential.BackupEligible,
		)
		if err != nil {
			return err
		}
		if value.IsNil() || value.String() == "" {
			return authentication.ErrPasskeyConcurrentUse
		}
		return insertAuthenticationSessionTX(ctx, tx, session)
	})
}

func consumeCeremonyTX(
	ctx context.Context,
	tx gdb.TX,
	ceremony authentication.Ceremony,
	identityID, sessionID string,
) error {
	query := `
UPDATE authentication_ceremonies
SET consumed_at = NOW()
WHERE id = ?
  AND kind = ?
  AND consumed_at IS NULL
  AND expires_at > NOW()
`
	args := []any{ceremony.ID, ceremony.Kind}
	if identityID != "" {
		query += " AND identity_id = ?"
		args = append(args, identityID)
	}
	if sessionID != "" {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	query += " RETURNING id"
	value, err := tx.GetValue(query, args...)
	if err != nil {
		return err
	}
	if value.IsNil() || value.String() == "" {
		return authentication.ErrCeremonyConsumed
	}
	return nil
}

func passkeyData(value authentication.PasskeyCredential) g.Map {
	transports := value.Transports
	if transports == nil {
		transports = []string{}
	}
	return g.Map{
		"id": value.ID, "identity_id": value.IdentityID, "rp_id": value.RPID,
		"credential_id": value.CredentialID, "public_key": value.PublicKey,
		"public_key_algorithm": value.PublicKeyAlgorithm,
		"transports":           transports, "attachment": value.Attachment,
		"attestation_type": value.AttestationType, "attestation_format": value.AttestationFormat,
		"aaguid": value.AAGUID, "sign_count": value.SignCount,
		"clone_warning": value.CloneWarning, "flags": value.Flags,
		"user_verified_at_registration": value.UserVerifiedAtRegistration,
		"backup_eligible":               value.BackupEligible, "backup_state": value.BackupState,
		"attestation_client_data_json":   value.AttestationClientDataJSON,
		"attestation_client_data_hash":   value.AttestationClientDataHash,
		"attestation_authenticator_data": value.AttestationAuthenticatorData,
		"attestation_object":             value.AttestationObject,
		"status":                         value.Status, "label": value.Label, "version": value.Version,
		"created_at": value.CreatedAt, "updated_at": value.UpdatedAt,
	}
}

type passkeyRow struct {
	ID                           string
	IdentityID                   string
	RPID                         string
	CredentialID                 []byte
	PublicKey                    []byte
	PublicKeyAlgorithm           int64
	Transports                   []string
	Attachment                   string
	AttestationType              string
	AttestationFormat            string
	AAGUID                       []byte
	SignCount                    uint32
	CloneWarning                 bool
	Flags                        byte
	UserVerifiedAtRegistration   bool
	BackupEligible               bool
	BackupState                  bool
	AttestationClientDataJSON    []byte
	AttestationClientDataHash    []byte
	AttestationAuthenticatorData []byte
	AttestationObject            []byte
	Status                       string
	Label                        string
	Version                      int64
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	LastUsedAt                   *time.Time
}

func (row passkeyRow) credential() authentication.PasskeyCredential {
	return authentication.PasskeyCredential{
		ID: row.ID, IdentityID: row.IdentityID, RPID: row.RPID,
		CredentialID: row.CredentialID, PublicKey: row.PublicKey,
		PublicKeyAlgorithm: row.PublicKeyAlgorithm, Transports: row.Transports,
		Attachment: row.Attachment, AttestationType: row.AttestationType,
		AttestationFormat: row.AttestationFormat, AAGUID: row.AAGUID,
		SignCount: row.SignCount, CloneWarning: row.CloneWarning, Flags: row.Flags,
		UserVerifiedAtRegistration: row.UserVerifiedAtRegistration,
		BackupEligible:             row.BackupEligible, BackupState: row.BackupState,
		AttestationClientDataJSON:    row.AttestationClientDataJSON,
		AttestationClientDataHash:    row.AttestationClientDataHash,
		AttestationAuthenticatorData: row.AttestationAuthenticatorData,
		AttestationObject:            row.AttestationObject, Status: row.Status,
		Label: row.Label, Version: row.Version, CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt, LastUsedAt: row.LastUsedAt,
	}
}

type ceremonyRow struct {
	ID              string
	Kind            string
	IdentityID      string
	SessionID       string
	ChallengeDigest []byte
	LibraryState    []byte
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
	FailedAttempts  int
	CreatedAt       time.Time
}

func (row ceremonyRow) ceremony() authentication.Ceremony {
	return authentication.Ceremony{
		ID: row.ID, Kind: authentication.CeremonyKind(row.Kind),
		IdentityID: row.IdentityID, SessionID: row.SessionID,
		ChallengeDigest: row.ChallengeDigest, LibraryState: row.LibraryState,
		ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt,
		FailedAttempts: row.FailedAttempts, CreatedAt: row.CreatedAt,
	}
}

var _ authentication.PasskeyStore = (*PG)(nil)
