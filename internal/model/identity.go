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

type Profile struct {
	IdentityID  string `orm:"identity_id"`
	Username    string `orm:"username"`
	DisplayName string `orm:"display_name"`
	AvatarURL   string `orm:"avatar_url"`
	Locale      string `orm:"locale"`
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
