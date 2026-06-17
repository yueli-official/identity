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
	ID            string
	Email         string // canonical
	EmailVerified bool
	Status        Status
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Profile struct {
	IdentityID  string
	Username    string
	DisplayName string
	AvatarURL   string
	Locale      string
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
