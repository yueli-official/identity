package model

import "time"

// GuestSession is the durable Identity record behind one opaque browser
// handle. ID is also the stable Guest Subject identifier used by consumers.
type GuestSession struct {
	ID                string     `orm:"id"`
	TokenHash         string     `orm:"token_hash"`
	ClientID          string     `orm:"client_id"`
	CreatedAt         time.Time  `orm:"created_at"`
	LastSeen          time.Time  `orm:"last_seen"`
	ExpiresAt         time.Time  `orm:"expires_at"`
	ClaimedIdentityID string     `orm:"claimed_identity_id"`
	ClaimedAt         *time.Time `orm:"claimed_at"`
	RevokedAt         *time.Time `orm:"revoked_at"`
}
