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
	Locale      string
}

// UpdateProfile validates and persists the editable profile fields for an
// identity, then returns the freshly stored profile. display_name is required;
// the rest are optional and trimmed.
func (s *Service) UpdateProfile(ctx context.Context, identityID string, in ProfileUpdate) (model.Profile, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Username = strings.TrimSpace(in.Username)
	in.AvatarURL = strings.TrimSpace(in.AvatarURL)
	in.Locale = strings.TrimSpace(in.Locale)
	if in.DisplayName == "" {
		return model.Profile{}, iderr.InvalidProfile("display name required")
	}
	if err := s.store.UpdateProfile(ctx, identityID, repo.ProfileUpdate(in)); err != nil {
		return model.Profile{}, err
	}
	s.audit(ctx, AuditEvent{Event: EvProfileUpdated, ActorID: identityID, TargetID: identityID})
	return s.store.GetProfile(ctx, identityID)
}
