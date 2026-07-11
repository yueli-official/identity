package model

import "time"

// OIDCClient is a registered relying party (consumer site).
type OIDCClient struct {
	ID                     string
	Public                 bool
	SecretHash             string
	SecretRef              string
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	Audiences              []string
	GrantTypes             []string
	ResponseTypes          []string
	Scopes                 []string
}

type KeyStatus string

const (
	KeyActive  KeyStatus = "active"
	KeyNext    KeyStatus = "next"
	KeyRetired KeyStatus = "retired"
)

// SigningKey is an RS256 key pair (PEM) used to sign tokens.
type SigningKey struct {
	KID        string    `orm:"kid"`
	Alg        string    `orm:"alg"`
	PrivatePEM string    `orm:"private_pem"`
	PublicPEM  string    `orm:"public_pem"`
	Status     KeyStatus `orm:"status"`
	CreatedAt  time.Time `orm:"created_at"`
}
