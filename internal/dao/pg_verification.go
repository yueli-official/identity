package dao

import (
	"context"
	"errors"

	"platform/services/identity/internal/repo"
)

// CreateVerification is a placeholder to be implemented in the PG slice.
func (p *PG) CreateVerification(ctx context.Context, in repo.NewVerificationInput) error {
	return errors.New("not implemented")
}

// ConsumeVerification is a placeholder to be implemented in the PG slice.
func (p *PG) ConsumeVerification(ctx context.Context, tokenHash, purpose string) (repo.VerificationRecord, error) {
	return repo.VerificationRecord{}, errors.New("not implemented")
}

// SetEmailVerified is a placeholder to be implemented in the PG slice.
func (p *PG) SetEmailVerified(ctx context.Context, identityID string, verified bool) error {
	return errors.New("not implemented")
}

// UpdatePasswordHash is a placeholder to be implemented in the PG slice.
func (p *PG) UpdatePasswordHash(ctx context.Context, identityID, passwordHash string) error {
	return errors.New("not implemented")
}
