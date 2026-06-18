package repo

// Composite assembles the full Store from separate IdentityRepo / OAuthRepo /
// SessionStore / LoginThrottle implementations (e.g. Postgres + Redis).
// IdentityRepo and OAuthRepo are typically the same PG-backed value.
type Composite struct {
	IdentityRepo
	OAuthRepo
	SessionStore
	LoginThrottle
}

// NewComposite returns a Store backed by the given implementations. The identity
// repo doubles as the OAuthRepo (both back onto the same PG database).
func NewComposite(i interface {
	IdentityRepo
	OAuthRepo
}, s SessionStore, l LoginThrottle) Store {
	return Composite{IdentityRepo: i, OAuthRepo: i, SessionStore: s, LoginThrottle: l}
}
