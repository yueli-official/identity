package dao

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"platform/services/identity/internal/repo"
)

// CreateVerification inserts an issued email token row (stored hashed, with TTL).
func (p *PG) CreateVerification(ctx context.Context, in repo.NewVerificationInput) error {
	_, err := p.db.Model("email_verifications").Ctx(ctx).Data(g.Map{
		"identity_id": in.IdentityID,
		"email":       in.Email,
		"purpose":     in.Purpose,
		"token_hash":  in.TokenHash,
		"expires_at":  in.ExpiresAt,
	}).Insert()
	return err
}

// ConsumeVerification atomically marks a still-valid token used and returns it.
// The single UPDATE ... RETURNING only touches a row that is unused, unexpired,
// and matches both the hash and the purpose, so single-use is race-free.
// Returns repo.ErrVerificationInvalid when no such row exists.
func (p *PG) ConsumeVerification(ctx context.Context, tokenHash, purpose string) (repo.VerificationRecord, error) {
	row, err := p.db.GetOne(ctx,
		`UPDATE email_verifications SET used_at = now()
		 WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now()
		 RETURNING identity_id, email`,
		tokenHash, purpose,
	)
	if err != nil {
		return repo.VerificationRecord{}, err
	}
	if len(row) == 0 {
		return repo.VerificationRecord{}, repo.ErrVerificationInvalid
	}
	return repo.VerificationRecord{
		IdentityID: row["identity_id"].String(),
		Email:      row["email"].String(),
	}, nil
}

// SetEmailVerified flips the identity's email_verified flag.
func (p *PG) SetEmailVerified(ctx context.Context, identityID string, verified bool) error {
	_, err := p.db.Model("identities").Ctx(ctx).
		Where("id", identityID).
		Update(g.Map{"email_verified": verified, "updated_at": gtime.Now()})
	return err
}

// UpdatePasswordHash replaces the identity's bcrypt password hash.
func (p *PG) UpdatePasswordHash(ctx context.Context, identityID, passwordHash string) error {
	_, err := p.db.Model("credentials_password").Ctx(ctx).
		Where("identity_id", identityID).
		Update(g.Map{"password_hash": passwordHash, "updated_at": gtime.Now()})
	return err
}

// Compile-time interface assertion.
var _ repo.VerificationRepo = (*PG)(nil)
