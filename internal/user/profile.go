package user

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	MaxDisplayNameRunes = 80
	MaxBioRunes         = 500
	MaxLocaleBytes      = 35
	MaxSocialLinks      = 8
	MaxSocialLabelRunes = 32
)

var (
	ErrDisplayNameRequired = errors.New("display name is required")
	ErrDisplayNameTooLong  = errors.New("display name is too long")
	ErrHandleInvalid       = errors.New("handle is invalid")
	ErrBioTooLong          = errors.New("bio is too long")
	ErrLocaleTooLong       = errors.New("locale is too long")
	ErrSocialLinksInvalid  = errors.New("social links are invalid")
)

// ProfileUpdate contains only fields owned by the general profile form.
// Avatar and cover references are committed by the Asset workflow; arbitrary
// social URLs require their own verified-link contract and are not accepted here.
type ProfileUpdate struct {
	DisplayName string
	Handle      string
	Bio         string
	Locale      string
}

type SocialLink struct {
	Label string
	URL   string
}

func NormalizeSocialLinks(input []SocialLink) ([]SocialLink, error) {
	if len(input) > MaxSocialLinks {
		return nil, ErrSocialLinksInvalid
	}
	out := make([]SocialLink, 0, len(input))
	labels := make(map[string]struct{}, len(input))
	for _, link := range input {
		link.Label = strings.TrimSpace(link.Label)
		link.URL = strings.TrimSpace(link.URL)
		if link.Label == "" || utf8.RuneCountInString(link.Label) > MaxSocialLabelRunes || link.URL == "" {
			return nil, ErrSocialLinksInvalid
		}
		parsed, err := url.Parse(link.URL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
			return nil, ErrSocialLinksInvalid
		}
		labelKey := strings.ToLower(link.Label)
		if _, duplicate := labels[labelKey]; duplicate {
			return nil, ErrSocialLinksInvalid
		}
		labels[labelKey] = struct{}{}
		out = append(out, link)
	}
	return out, nil
}

func NormalizeProfileUpdate(input ProfileUpdate) (ProfileUpdate, error) {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Bio = strings.TrimSpace(input.Bio)
	input.Locale = strings.TrimSpace(input.Locale)
	if input.DisplayName == "" {
		return ProfileUpdate{}, ErrDisplayNameRequired
	}
	if utf8.RuneCountInString(input.DisplayName) > MaxDisplayNameRunes {
		return ProfileUpdate{}, ErrDisplayNameTooLong
	}
	handle, err := NormalizeOptionalHandle(input.Handle)
	if err != nil {
		return ProfileUpdate{}, ErrHandleInvalid
	}
	input.Handle = string(handle)
	if utf8.RuneCountInString(input.Bio) > MaxBioRunes {
		return ProfileUpdate{}, ErrBioTooLong
	}
	if len(input.Locale) > MaxLocaleBytes {
		return ProfileUpdate{}, ErrLocaleTooLong
	}
	return input, nil
}
