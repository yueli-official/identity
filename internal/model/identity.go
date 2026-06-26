// Package model holds identity-service domain entities (DB-agnostic).
package model

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusDeleted  Status = "deleted"
)

type Identity struct {
	ID            string    `orm:"id"`
	Email         string    `orm:"email"` // canonical
	EmailVerified bool      `orm:"email_verified"`
	Status        Status    `orm:"status"`
	CreatedAt     time.Time `orm:"created_at"`
	UpdatedAt     time.Time `orm:"updated_at"`
}

// SocialLink is one labelled external link on a profile (e.g. {GitHub, https://…}).
type SocialLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Profile struct {
	IdentityID  string       `orm:"identity_id"`
	Username    string       `orm:"username"`
	DisplayName string       `orm:"display_name"`
	AvatarURL   string       `orm:"avatar_url"`
	CoverURL    string       `orm:"cover_url"`
	Bio         string       `orm:"bio"`
	SocialLinks []SocialLink `orm:"social_links"`
	Locale      string       `orm:"locale"`
	// asset ids behind avatar_url / cover_url, so a replacement can delete the old.
	AvatarAssetID string `orm:"avatar_asset_id"`
	CoverAssetID  string `orm:"cover_asset_id"`
}

// Session is an IdP self-hosted login session (Redis-backed).
type Session struct {
	ID         string
	IdentityID string
	CreatedAt  time.Time
	LastSeen   time.Time
	UserAgent  string
	IP         string
	Device     string
}
