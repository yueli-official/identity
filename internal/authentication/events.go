package authentication

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SecurityEventKind string

const (
	EventPasskeyRegistered SecurityEventKind = "passkey.registered"
	EventPasskeyLogin      SecurityEventKind = "passkey.login"
	EventPasskeyRenamed    SecurityEventKind = "passkey.renamed"
	EventPasskeyRevoked    SecurityEventKind = "passkey.revoked"
	EventTOTPEnrolled      SecurityEventKind = "totp.enrolled"
	EventTOTPRevoked       SecurityEventKind = "totp.revoked"
	EventRecoveryGenerated SecurityEventKind = "recovery.generated"
	EventRecoveryUsed      SecurityEventKind = "recovery.used"
	EventTOTPLogin         SecurityEventKind = "totp.login"
)

type SecurityEvent struct {
	ID           string
	Kind         SecurityEventKind
	IdentityID   string
	CredentialID string
	SessionID    string
	Label        string
	OccurredAt   time.Time
	Detail       map[string]any
}

// SecurityEventSink receives already-committed authentication events. It must
// never make credential/session success depend on an audit or notification
// transport; implementations own retry/logging semantics.
type SecurityEventSink interface {
	RecordAuthenticationEvent(context.Context, SecurityEvent)
}

func (module *Module) SetSecurityEventSink(sink SecurityEventSink) {
	if module != nil {
		module.events = sink
	}
}

func (module *Module) recordEvent(ctx context.Context, event SecurityEvent) {
	if module.events == nil {
		return
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = module.now().UTC()
	}
	module.events.RecordAuthenticationEvent(ctx, event)
}
