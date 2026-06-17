package repo

// Composite assembles the full Store from separate IdentityRepo / SessionStore /
// LoginThrottle implementations (e.g. Postgres + Redis).
type Composite struct {
	IdentityRepo
	SessionStore
	LoginThrottle
}

// NewComposite returns a Store backed by the given implementations.
func NewComposite(i IdentityRepo, s SessionStore, l LoginThrottle) Store {
	return Composite{IdentityRepo: i, SessionStore: s, LoginThrottle: l}
}
