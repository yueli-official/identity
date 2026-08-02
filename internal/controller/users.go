package controller

import (
	"context"
	"strings"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/model"
)

func (c *Controller) PublicUser(ctx context.Context, req *v1.GetUserReq) (*v1.GetUserRes, error) {
	result, err := c.svc.PublicUser(ctx, req.UserKey)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserRes{User: toPublicUser(result)}, nil
}

func (c *Controller) PublicUserByHandle(ctx context.Context, req *v1.GetUserByHandleReq) (*v1.GetUserByHandleRes, error) {
	result, err := c.svc.PublicUserByHandle(ctx, req.Handle)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserByHandleRes{User: toPublicUser(result)}, nil
}

func (c *Controller) PublicUsers(ctx context.Context, req *v1.BatchUsersReq) (*v1.BatchUsersRes, error) {
	keys := make([]string, 0)
	for _, key := range strings.Split(req.IDs, ",") {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	users, err := c.svc.PublicUsers(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]v1.PublicUser, 0, len(users))
	for _, value := range users {
		out = append(out, toPublicUser(value))
	}
	return &v1.BatchUsersRes{Users: out}, nil
}

func toPublicUser(value model.PublicUser) v1.PublicUser {
	return v1.PublicUser{
		UserKey: value.UserKey, Handle: value.Handle, DisplayName: value.DisplayName,
		Avatar: mediaRef(value.AvatarMediaKey), Cover: mediaRef(value.CoverMediaKey), Bio: value.Bio,
		SocialLinks: socialToDTO(value.SocialLinks),
	}
}

func mediaRef(mediaKey string) *v1.MediaRef {
	if mediaKey == "" {
		return nil
	}
	return &v1.MediaRef{MediaKey: mediaKey}
}

func socialToDTO(in []model.SocialLink) []v1.SocialLinkDTO {
	out := make([]v1.SocialLinkDTO, 0, len(in))
	for _, link := range in {
		out = append(out, v1.SocialLinkDTO{Label: link.Label, URL: link.URL})
	}
	return out
}

func socialFromDTO(in []v1.SocialLinkDTO) []model.SocialLink {
	out := make([]model.SocialLink, 0, len(in))
	for _, link := range in {
		out = append(out, model.SocialLink{Label: link.Label, URL: link.URL})
	}
	return out
}
