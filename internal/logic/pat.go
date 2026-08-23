package logic

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/pat"
	"github.com/yueli-official/identity/internal/repo"
)

// PATVerification is returned by VerifyPAT on success.
type PATVerification struct {
	UserKey   string
	Scopes    []string
	ExpiresAt *time.Time
}

const (
	PATScopeProfileRead = "identity:profile:read"
	PATScopeEmailRead   = "identity:email:read"
)

type PATScopeDefinition struct {
	Key         string
	Label       string
	Description string
}

var patScopeCatalog = []PATScopeDefinition{
	{Key: PATScopeProfileRead, Label: "读取个人资料", Description: "读取昵称、公开主页地址、简介、头像和公开链接。"},
	{Key: PATScopeEmailRead, Label: "读取账户邮箱", Description: "读取登录邮箱和邮箱验证状态。"},
}

func PATScopes() []PATScopeDefinition {
	result := make([]PATScopeDefinition, len(patScopeCatalog))
	copy(result, patScopeCatalog)
	return result
}

func validPATScope(scope string) bool {
	for _, definition := range patScopeCatalog {
		if definition.Key == scope {
			return true
		}
	}
	return false
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
		return "", repo.PATRow{}, iderr.PATScopesTooMany(50)
	}
	seenScopes := make(map[string]bool, len(scopes))
	normalizedScopes := make([]string, 0, len(scopes))
	for index, sc := range scopes {
		sc = strings.TrimSpace(sc)
		if sc == "" || strings.ContainsAny(sc, " \t\r\n") || !validPATScope(sc) {
			return "", repo.PATRow{}, iderr.PATScopeInvalid(index)
		}
		if !seenScopes[sc] {
			seenScopes[sc] = true
			normalizedScopes = append(normalizedScopes, sc)
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
		Scopes:      normalizedScopes,
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
		Detail:   map[string]any{"pat_id": id, "name": trimmedName, "scopes": normalizedScopes},
	})

	return plaintext, row, nil
}

type PATUserInfo struct {
	UserKey       string
	Email         *string
	EmailVerified *bool
	Profile       *model.Profile
}

func (s *Service) PATUserInfo(ctx context.Context, presented string) (PATUserInfo, error) {
	verification, err := s.VerifyPAT(ctx, presented)
	if err != nil {
		return PATUserInfo{}, err
	}
	profileAllowed := containsPATScope(verification.Scopes, PATScopeProfileRead)
	emailAllowed := containsPATScope(verification.Scopes, PATScopeEmailRead)
	if !profileAllowed && !emailAllowed {
		return PATUserInfo{}, iderr.PATInsufficientScope()
	}
	identity, err := s.store.GetByUserKey(ctx, verification.UserKey)
	if err != nil {
		return PATUserInfo{}, iderr.PATInvalid()
	}
	result := PATUserInfo{UserKey: identity.UserKey}
	if emailAllowed {
		email := identity.Email
		verified := identity.EmailVerified
		result.Email = &email
		result.EmailVerified = &verified
	}
	if profileAllowed {
		profile, err := s.store.GetProfile(ctx, identity.ID)
		if err != nil {
			return PATUserInfo{}, err
		}
		result.Profile = &profile
	}
	return result, nil
}

func containsPATScope(scopes []string, scope string) bool {
	for _, candidate := range scopes {
		if candidate == scope {
			return true
		}
	}
	return false
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

	// 3. A PAT cannot outlive the account's active lifecycle. Status changes
	// must take effect even when the token itself has no expiry.
	identity, err := s.store.GetByID(ctx, row.IdentityID)
	if err != nil || identity.Status != model.StatusActive {
		return PATVerification{}, iderr.PATInvalid()
	}

	// 4. Check expiry: !After means at-or-past expiry boundary.
	now := s.now()
	if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
		return PATVerification{}, iderr.PATExpired()
	}
	for _, scope := range row.Scopes {
		if !validPATScope(scope) {
			return PATVerification{}, iderr.PATInvalid()
		}
	}

	// 5. Throttled touch (best-effort: at most once per minute).
	if row.LastUsedAt == nil || now.Sub(*row.LastUsedAt) >= time.Minute {
		if err := s.store.TouchPATLastUsed(ctx, row.ID, now); err != nil {
			g.Log().Errorf(ctx, "pat: touch last_used %d failed: %v", row.ID, err)
		}
	}

	return PATVerification{
		UserKey:   identity.UserKey,
		Scopes:    row.Scopes,
		ExpiresAt: row.ExpiresAt,
	}, nil
}
