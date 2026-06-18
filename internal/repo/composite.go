package repo

// Composite assembles the full Store from separate IdentityRepo / OAuthRepo /
// VerificationRepo / RoleRepo / SessionStore / LoginThrottle implementations
// (e.g. Postgres + Redis). IdentityRepo, OAuthRepo, VerificationRepo and
// RoleRepo are typically the same PG-backed value.
type Composite struct {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
	SessionStore
	LoginThrottle
}

// NewComposite returns a Store backed by the given implementations. The identity
// repo doubles as the OAuthRepo, VerificationRepo and RoleRepo (all back onto the
// same PG database).
func NewComposite(i interface {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
}, s SessionStore, l LoginThrottle) Store {
	return Composite{IdentityRepo: i, OAuthRepo: i, VerificationRepo: i, RoleRepo: i, SessionStore: s, LoginThrottle: l}
}
