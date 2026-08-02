package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
	"github.com/yueli-official/identity/internal/user"
)

// ProfileUpdate carries the user-editable profile fields from the account UI.
type ProfileUpdate struct {
	DisplayName string
	Handle      string
	Bio         string
	Locale      string
}

// UpdateProfile validates and persists the editable profile fields for an
// identity, then returns the freshly stored profile. display_name is required;
// the rest are optional and trimmed. Asset references and social links have
// dedicated mutation boundaries and are not accepted here.
func (s *Service) UpdateProfile(ctx context.Context, identityID string, in ProfileUpdate) (model.Profile, error) {
	normalized, err := user.NormalizeProfileUpdate(user.ProfileUpdate(in))
	if err != nil {
		return model.Profile{}, mapProfileValidation(err)
	}
	if err := s.store.UpdateProfile(ctx, identityID, repo.ProfileUpdate(normalized)); err != nil {
		if errors.Is(err, repo.ErrHandleUnavailable) {
			return model.Profile{}, iderr.HandleUnavailable()
		}
		return model.Profile{}, err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return s.store.GetProfile(ctx, identityID)
}

func (s *Service) UpdateSocialLinks(ctx context.Context, identityID string, links []model.SocialLink) ([]model.SocialLink, error) {
	input := make([]user.SocialLink, 0, len(links))
	for _, link := range links {
		input = append(input, user.SocialLink{Label: link.Label, URL: link.URL})
	}
	normalized, err := user.NormalizeSocialLinks(input)
	if err != nil {
		return nil, iderr.InvalidProfile(iderr.ProfileReasonSocialLinksInvalid)
	}
	stored := make([]model.SocialLink, 0, len(normalized))
	for _, link := range normalized {
		stored = append(stored, model.SocialLink{Label: link.Label, URL: link.URL})
	}
	if err := s.store.SetProfileSocialLinks(ctx, identityID, stored); err != nil {
		return nil, err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return stored, nil
}

func mapProfileValidation(err error) error {
	switch {
	case errors.Is(err, user.ErrDisplayNameRequired):
		return iderr.InvalidProfile(iderr.ProfileReasonDisplayNameRequired)
	case errors.Is(err, user.ErrDisplayNameTooLong):
		return iderr.InvalidProfile(iderr.ProfileReasonDisplayNameTooLong)
	case errors.Is(err, user.ErrHandleInvalid):
		return iderr.InvalidProfile(iderr.ProfileReasonHandleInvalid)
	case errors.Is(err, user.ErrBioTooLong):
		return iderr.InvalidProfile(iderr.ProfileReasonBioTooLong)
	case errors.Is(err, user.ErrLocaleTooLong):
		return iderr.InvalidProfile(iderr.ProfileReasonLocaleTooLong)
	case errors.Is(err, user.ErrSocialLinksInvalid):
		return iderr.InvalidProfile(iderr.ProfileReasonSocialLinksInvalid)
	default:
		return err
	}
}

// SetProfileImage commits a single uploaded image (avatar or cover) to the
// caller's profile, recording both its public media key and internal asset id. kind is
// "avatar" or "cover".
func (s *Service) SetProfileImage(ctx context.Context, identityID, kind, mediaKey, assetID string) error {
	if err := s.store.SetProfileImage(ctx, identityID, kind, mediaKey, assetID); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return nil
}

// PublicUser resolves a stable public user key without exposing the internal
// identity UUID. Missing, disabled and deleted users share the same not-found
// response.
func (s *Service) PublicUser(ctx context.Context, userKey string) (model.PublicUser, error) {
	if _, err := user.ParsePublicKey(strings.TrimSpace(userKey)); err != nil {
		return model.PublicUser{}, iderr.IdentityNotFound()
	}
	result, err := s.store.GetPublicUserByKey(ctx, userKey)
	if errors.Is(err, repo.ErrIdentityMissing) {
		return model.PublicUser{}, iderr.IdentityNotFound()
	}
	return result, err
}

func (s *Service) PublicUserByHandle(ctx context.Context, value string) (model.PublicUser, error) {
	handle, err := user.NormalizeHandle(value)
	if err != nil {
		return model.PublicUser{}, iderr.IdentityNotFound()
	}
	result, err := s.store.GetPublicUserByHandle(ctx, string(handle))
	if errors.Is(err, repo.ErrIdentityMissing) {
		return model.PublicUser{}, iderr.IdentityNotFound()
	}
	return result, err
}

// PublicUsers resolves at most 100 distinct public keys in one repository call.
// Missing users are omitted while infrastructure failures are returned.
func (s *Service) PublicUsers(ctx context.Context, userKeys []string) ([]model.PublicUser, error) {
	if len(userKeys) > 100 {
		return nil, iderr.InvalidProfile(iderr.ProfileReasonBatchTooLarge)
	}
	seen := make(map[string]struct{}, len(userKeys))
	canonical := make([]string, 0, len(userKeys))
	for _, value := range userKeys {
		value = strings.TrimSpace(value)
		if _, err := user.ParsePublicKey(value); err != nil {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		canonical = append(canonical, value)
	}
	return s.store.GetPublicUsersByKeys(ctx, canonical)
}
