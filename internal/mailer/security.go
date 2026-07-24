package mailer

import (
	"context"
	"time"
)

type SecurityAlert struct {
	EventID    string
	To         string
	Action     string
	Device     string
	IP         string
	OccurredAt time.Time
	AccountURL string
}

type SecurityNotifier interface {
	SendSecurityAlert(context.Context, SecurityAlert) error
}
