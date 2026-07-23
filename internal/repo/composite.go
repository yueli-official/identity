package repo

// Composite assembles the full Store from separate IdentityRepo / OAuthRepo /
// VerificationRepo / RoleRepo / AuditRepo / PATRepo / SessionStore /
// VerificationThrottle
// implementations (e.g. Postgres + Redis). IdentityRepo, OAuthRepo,
// VerificationRepo, RoleRepo, AuditRepo, and PATRepo are typically the same
// PG-backed value.
type Composite struct {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
	AuditRepo
	PATRepo
	SessionStore
	VerificationThrottle
}

// NewComposite returns a Store backed by the given implementations. The identity
// repo doubles as the OAuthRepo, VerificationRepo, RoleRepo, AuditRepo, and
// PATRepo (all back onto the same PG database).
func NewComposite(i interface {
	IdentityRepo
	OAuthRepo
	VerificationRepo
	RoleRepo
	AuditRepo
	PATRepo
}, s SessionStore, throttle VerificationThrottle) Store {
	return Composite{
		IdentityRepo:         i,
		OAuthRepo:            i,
		VerificationRepo:     i,
		RoleRepo:             i,
		AuditRepo:            i,
		PATRepo:              i,
		SessionStore:         s,
		VerificationThrottle: throttle,
	}
}
