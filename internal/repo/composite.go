package repo

// Composite assembles the full Store from separate IdentityRepo / OAuthRepo /
// VerificationRepo / RoleRepo / AuditRepo / SessionStore / LoginThrottle
// implementations (e.g. Postgres + Redis). IdentityRepo, OAuthRepo,
// VerificationRepo, RoleRepo, and AuditRepo are typically the same PG-backed value.
type Composite struct {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
	AuditRepo
	SessionStore
	LoginThrottle
}

// NewComposite returns a Store backed by the given implementations. The identity
// repo doubles as the OAuthRepo, VerificationRepo, RoleRepo, and AuditRepo (all
// back onto the same PG database).
func NewComposite(i interface {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
	AuditRepo
}, s SessionStore, l LoginThrottle) Store {
	return Composite{
		IdentityRepo:     i,
		OAuthRepo:        i,
		VerificationRepo: i,
		RoleRepo:         i,
		AuditRepo:        i,
		SessionStore:     s,
		LoginThrottle:    l,
	}
}
