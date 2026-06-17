package model

import "time"

// OIDCClient is a registered relying party (consumer site).
type OIDCClient struct {
	ID            string
	Public        bool
	SecretHash    string
	RedirectURIs  []string
	GrantTypes    []string
	ResponseTypes []string
	Scopes        []string
}

type KeyStatus string

const (
	KeyActive  KeyStatus = "active"
	KeyNext    KeyStatus = "next"
	KeyRetired KeyStatus = "retired"
)

// SigningKey is an RS256 key pair (PEM) used to sign tokens.
type SigningKey struct {
	KID        string
	Alg        string
	PrivatePEM string
	PublicPEM  string
	Status     KeyStatus
	CreatedAt  time.Time
}
