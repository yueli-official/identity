package logic

import (
	"time"

	"platform/services/identity/internal/repo"
)

// Config holds tunables (rate-limit, session TTL); values are plan-level defaults.
type Config struct {
	SessionIdleTTL  time.Duration
	LoginMaxFails   int
	LoginFailWindow time.Duration
	LoginLockFor    time.Duration
	IPMaxFails      int
}

func DefaultConfig() Config {
	return Config{
		SessionIdleTTL:  24 * time.Hour,
		LoginMaxFails:   5,
		LoginFailWindow: 15 * time.Minute,
		LoginLockFor:    15 * time.Minute,
		IPMaxFails:      50,
	}
}

// Service is the identity-service application layer.
type Service struct {
	store repo.Store
	cfg   Config
	now   func() time.Time
}

func New(store repo.Store, cfg Config) *Service {
	return &Service{store: store, cfg: cfg, now: time.Now}
}
