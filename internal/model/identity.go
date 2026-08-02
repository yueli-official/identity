// Package model holds identity-service domain entities (DB-agnostic).
package model

import (
	"time"

	"github.com/yueli-official/identity/internal/authentication"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusDeleted  Status = "deleted"
)

type Identity struct {
	ID            string    `orm:"id"`
	UserKey       string    `orm:"user_key"`
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
	IdentityID     string       `orm:"identity_id"`
	Handle         string       `orm:"handle"`
	DisplayName    string       `orm:"display_name"`
	AvatarMediaKey string       `orm:"avatar_media_key"`
	CoverMediaKey  string       `orm:"cover_media_key"`
	Bio            string       `orm:"bio"`
	SocialLinks    []SocialLink `orm:"social_links"`
	Locale         string       `orm:"locale"`
	// Internal Asset IDs let a replacement delete the prior object; they never
	// cross the public profile contract.
	AvatarAssetID string `orm:"avatar_asset_id"`
	CoverAssetID  string `orm:"cover_asset_id"`
}

// PublicUser is the public, status-filtered projection returned by the User
// module. It deliberately excludes the internal UUID, email, roles and status.
type PublicUser struct {
	UserKey        string       `orm:"user_key"`
	Handle         string       `orm:"handle"`
	DisplayName    string       `orm:"display_name"`
	AvatarMediaKey string       `orm:"avatar_media_key"`
	CoverMediaKey  string       `orm:"cover_media_key"`
	Bio            string       `orm:"bio"`
	SocialLinks    []SocialLink `orm:"social_links"`
}

type Session = authentication.Session
