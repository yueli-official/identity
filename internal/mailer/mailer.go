// Package mailer sends transactional emails. Logic builds the action link;
// each impl renders subject + body and delivers it.
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
