package logic

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/pat"
	"platform/services/identity/internal/repo"
)

// PATVerification is returned by VerifyPAT on success.
type PATVerification struct {
	IdentityID string
	Scopes     []string
	ExpiresAt  *time.Time
}

// CreatePAT generates a new Personal Access Token for the given identity.
// It returns the plaintext token (shown once), the stored row, and any error.
func (s *Service) CreatePAT(ctx context.Context, identityID, name string, scopes []string, expiresInDays int) (plaintext string, row repo.PATRow, err error) {
	// 1. Validate name.
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" || len(trimmedName) > 100 {
		return "", repo.PATRow{}, iderr.PATNameRequired()
	}

	// 2. Validate scopes.
	if len(scopes) == 0 {
		return "", repo.PATRow{}, iderr.PATScopesRequired()
	}
	if len(scopes) > 50 {
		return "", repo.PATRow{}, iderr.PATScopeInvalid()
	}
	for _, sc := range scopes {
		trimmed := strings.TrimSpace(sc)
		if trimmed == "" || strings.ContainsAny(sc, " \t\r\n") {
			return "", repo.PATRow{}, iderr.PATScopeInvalid()
		}
	}

	// 3. Enforce per-user cap.
	n, err := s.store.CountPATByIdentity(ctx, identityID)
	if err != nil {
		return "", repo.PATRow{}, err
	}
	if n >= s.cfg.PATMaxPerUser {
		return "", repo.PATRow{}, iderr.PATLimitReached(s.cfg.PATMaxPerUser)
	}

	// 4. Generate token.
	plaintext, err = pat.Generate()
	if err != nil {
		return "", repo.PATRow{}, err
	}
	hash := pat.Hash(s.patKey, plaintext)
	prefix := pat.Display(plaintext)

	// 5. Build expiry.
	var expPtr *time.Time
	if expiresInDays > 0 {
		t := s.now().Add(time.Duration(expiresInDays) * 24 * time.Hour)
		expPtr = &t
	}

	// 6. Insert.
	row = repo.PATRow{
		IdentityID:  identityID,
		Name:        trimmedName,
		TokenHash:   hash,
		TokenPrefix: prefix,
		Scopes:      scopes,
		ExpiresAt:   expPtr,
		CreatedAt:   s.now(),
	}
	id, err := s.store.InsertPAT(ctx, row)
	if err != nil {
		return "", repo.PATRow{}, err
	}
	row.ID = id

	// 7. Audit (best-effort).
	s.audit(ctx, AuditEvent{
		Event:    EvPATCreated,
		ActorID:  identityID,
		TargetID: identityID,
		Detail:   map[string]any{"pat_id": id, "name": trimmedName, "scopes": scopes},
	})

	return plaintext, row, nil
}

// ListPAT returns all PATs for an identity, newest-first.
func (s *Service) ListPAT(ctx context.Context, identityID string) ([]repo.PATRow, error) {
	return s.store.ListPATByIdentity(ctx, identityID)
}

// RevokePAT deletes the token with the given id, enforcing identity ownership.
func (s *Service) RevokePAT(ctx context.Context, identityID string, id int64) error {
	deleted, err := s.store.DeletePAT(ctx, id, identityID)
	if err != nil {
		return err
	}
	if !deleted {
		return iderr.PATNotFound()
	}
	s.audit(ctx, AuditEvent{
		Event:    EvPATRevoked,
		ActorID:  identityID,
		TargetID: identityID,
		Detail:   map[string]any{"pat_id": id},
	})
	return nil
}

// VerifyPAT validates a presented plaintext PAT and returns the associated
// identity and scopes. It never audits (too noisy), but does a throttled touch
// of LastUsedAt (best-effort: a touch failure is logged, not returned).
func (s *Service) VerifyPAT(ctx context.Context, presented string) (PATVerification, error) {
	// 1. Parse prefix.
	token, ok := pat.Parse(presented)
	if !ok {
		return PATVerification{}, iderr.PATInvalid()
	}

	// 2. Look up by hash.
	hash := pat.Hash(s.patKey, token)
	row, found, err := s.store.GetPATByHash(ctx, hash)
	if err != nil {
		return PATVerification{}, err
	}
	if !found {
		return PATVerification{}, iderr.PATInvalid()
	}

	// 3. Check expiry: !After means at-or-past expiry boundary.
	if row.ExpiresAt != nil && !row.ExpiresAt.After(s.now()) {
		return PATVerification{}, iderr.PATExpired()
	}

	// 4. Throttled touch (best-effort: at most once per minute).
	if row.LastUsedAt == nil || s.now().Sub(*row.LastUsedAt) >= time.Minute {
		if err := s.store.TouchPATLastUsed(ctx, row.ID, s.now()); err != nil {
			g.Log().Errorf(ctx, "pat: touch last_used %d failed: %v", row.ID, err)
		}
	}

	return PATVerification{
		IdentityID: row.IdentityID,
		Scopes:     row.Scopes,
		ExpiresAt:  row.ExpiresAt,
	}, nil
}
