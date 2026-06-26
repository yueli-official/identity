package controller

import (
	"context"
	"strings"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/model"
)

// PublicProfile returns the public display subset of one identity's profile.
// A missing profile yields an empty public shell (no existence leak), never an error.
func (c *Controller) PublicProfile(ctx context.Context, req *v1.GetProfileReq) (*v1.GetProfileRes, error) {
	p, err := c.svc.PublicProfile(ctx, req.ID)
	if err != nil {
		return &v1.GetProfileRes{Profile: v1.ProfilePublic{ID: req.ID}}, nil
	}
	return &v1.GetProfileRes{Profile: toPublicProfile(p)}, nil
}

// PublicProfiles resolves a comma-separated list of ids to their public profiles.
func (c *Controller) PublicProfiles(ctx context.Context, req *v1.BatchProfilesReq) (*v1.BatchProfilesRes, error) {
	ids := make([]string, 0)
	for _, id := range strings.Split(req.IDs, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	ps := c.svc.PublicProfiles(ctx, ids)
	out := make([]v1.ProfilePublic, 0, len(ps))
	for _, p := range ps {
		out = append(out, toPublicProfile(p))
	}
	return &v1.BatchProfilesRes{Profiles: out}, nil
}

// ── shared model ↔ DTO conversions for social links / public profiles ────────

func toPublicProfile(p model.Profile) v1.ProfilePublic {
	return v1.ProfilePublic{
		ID:          p.IdentityID,
		DisplayName: p.DisplayName,
		AvatarURL:   p.AvatarURL,
		CoverURL:    p.CoverURL,
		Bio:         p.Bio,
		SocialLinks: socialToDTO(p.SocialLinks),
	}
}

func socialToDTO(in []model.SocialLink) []v1.SocialLinkDTO {
	out := make([]v1.SocialLinkDTO, 0, len(in))
	for _, l := range in {
		out = append(out, v1.SocialLinkDTO{Label: l.Label, URL: l.URL})
	}
	return out
}

func socialFromDTO(in []v1.SocialLinkDTO) []model.SocialLink {
	out := make([]model.SocialLink, 0, len(in))
	for _, l := range in {
		out = append(out, model.SocialLink{Label: l.Label, URL: l.URL})
	}
	return out
}
