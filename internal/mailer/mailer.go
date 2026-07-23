// Package mailer submits Identity's transactional scenes. Logic builds the
// action link; Notification owns rendering and delivery.
package mailer

import "context"

type Mailer interface {
	SendVerifyEmail(ctx context.Context, to, link string) error
	SendPasswordReset(ctx context.Context, to, link string) error
}

// HealthChecker performs a side-effect-free transport probe.
type HealthChecker interface {
	CheckHealth(context.Context) error
}
