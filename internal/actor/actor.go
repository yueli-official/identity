package actor

import "context"

type ctxKey struct{}

// Actor is request-scoped metadata for audit. IdentityID is the *caller* (the
// authenticated session principal); it is "" for unauthenticated / pre-auth flows.
type Actor struct {
	IP         string
	UserAgent  string
	RequestID  string
	IdentityID string
}

// With stores a in ctx, replacing any previously stored Actor.
func With(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// WithIdentity enriches an existing Actor (or a fresh one) with the caller identity.
func WithIdentity(ctx context.Context, identityID string) context.Context {
	a := From(ctx)
	a.IdentityID = identityID
	return With(ctx, a)
}

// From returns the Actor stored in ctx, or a zero Actor if absent (hermetic tests
// with no middleware get the zero value).
func From(ctx context.Context) Actor {
	if a, ok := ctx.Value(ctxKey{}).(Actor); ok {
		return a
	}
	return Actor{}
}
