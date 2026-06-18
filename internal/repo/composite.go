package repo

// Composite assembles the full Store from separate IdentityRepo / OAuthRepo /
// VerificationRepo / SessionStore / LoginThrottle implementations (e.g. Postgres
// + Redis). IdentityRepo, OAuthRepo and VerificationRepo are typically the same
// PG-backed value.
type Composite struct {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	SessionStore
	LoginThrottle
}

// NewComposite returns a Store backed by the given implementations. The identity
// repo doubles as the OAuthRepo and VerificationRepo (all back onto the same PG
// database).
func NewComposite(i interface {
	IdentityRepo
	OAuthRepo
	VerificationRepo
}, s SessionStore, l LoginThrottle) Store {
	return Composite{IdentityRepo: i, OAuthRepo: i, VerificationRepo: i, SessionStore: s, LoginThrottle: l}
}
