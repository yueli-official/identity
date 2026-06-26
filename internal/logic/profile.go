package logic

import (
	"context"
	"strings"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// ProfileUpdate carries the user-editable profile fields from the account UI.
type ProfileUpdate struct {
	DisplayName string
	Username    string
	AvatarURL   string
	CoverURL    string
	Bio         string
	SocialLinks []model.SocialLink
	Locale      string
}

// UpdateProfile validates and persists the editable profile fields for an
// identity, then returns the freshly stored profile. display_name is required;
// the rest are optional and trimmed. Social links with a blank URL are dropped.
func (s *Service) UpdateProfile(ctx context.Context, identityID string, in ProfileUpdate) (model.Profile, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Username = strings.TrimSpace(in.Username)
	in.AvatarURL = strings.TrimSpace(in.AvatarURL)
	in.CoverURL = strings.TrimSpace(in.CoverURL)
	in.Bio = strings.TrimSpace(in.Bio)
	in.Locale = strings.TrimSpace(in.Locale)
	if in.DisplayName == "" {
		return model.Profile{}, iderr.InvalidProfile("display name required")
	}
	links := make([]model.SocialLink, 0, len(in.SocialLinks))
	for _, l := range in.SocialLinks {
		l.Label = strings.TrimSpace(l.Label)
		l.URL = strings.TrimSpace(l.URL)
		if l.URL == "" {
			continue
		}
		links = append(links, l)
	}
	in.SocialLinks = links
	if err := s.store.UpdateProfile(ctx, identityID, repo.ProfileUpdate(in)); err != nil {
		return model.Profile{}, err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return s.store.GetProfile(ctx, identityID)
}

// SetProfileImage commits a single uploaded image (avatar or cover) to the
// caller's profile, recording both its public url and asset id. kind is
// "avatar" or "cover".
func (s *Service) SetProfileImage(ctx context.Context, identityID, kind, url, assetID string) error {
	if err := s.store.SetProfileImage(ctx, identityID, kind, url, assetID); err != nil {
		return err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return nil
}

// PublicProfile returns the public display subset of an identity's profile for
// cross-user resolution (author pages, bylines). Returns ErrIdentityMissing
// when the profile does not exist.
func (s *Service) PublicProfile(ctx context.Context, id string) (model.Profile, error) {
	return s.store.GetProfile(ctx, id)
}

// PublicProfiles resolves a batch of identity ids to their public profiles,
// skipping any that are missing. Dev-scale: a small loop, no N+1 concern.
func (s *Service) PublicProfiles(ctx context.Context, ids []string) []model.Profile {
	out := make([]model.Profile, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		p, err := s.store.GetProfile(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}
