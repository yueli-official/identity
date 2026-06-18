package logic

import (
	"time"

	"platform/services/identity/internal/mailer"
	"platform/services/identity/internal/pat"
	"platform/services/identity/internal/repo"
)

// Config holds tunables (rate-limit, session TTL); values are plan-level defaults.
type Config struct {
	SessionIdleTTL  time.Duration
	LoginMaxFails   int
	LoginFailWindow time.Duration
	LoginLockFor    time.Duration
	IPMaxFails      int

	// Email verification + password reset (milestone ⑤).
	AccountBaseURL     string        // base URL the action links point at
	VerifyTokenTTL     time.Duration // verify-email token lifetime
	ResetTokenTTL      time.Duration // password-reset token lifetime
	VerifyMaxReq       int           // max verify-email requests per window
	ResetMaxReq        int           // max password-reset requests per window
	VerifyResetWindow  time.Duration // throttle counting window (verify + reset)
	VerifyResetLockFor time.Duration // throttle lockout duration (verify + reset)

	// Personal Access Tokens (PAT).
	PATHMACSecret string // HKDF secret; empty → dev fallback (codec handles it)
	PATMaxPerUser int    // max PATs per identity (0 → default 20)
}

func DefaultConfig() Config {
	return Config{
		SessionIdleTTL:  24 * time.Hour,
		LoginMaxFails:   5,
		LoginFailWindow: 15 * time.Minute,
		LoginLockFor:    15 * time.Minute,
		IPMaxFails:      50,

		AccountBaseURL:     "http://localhost:3000",
		VerifyTokenTTL:     24 * time.Hour,
		ResetTokenTTL:      time.Hour,
		VerifyMaxReq:       5,
		ResetMaxReq:        5,
		VerifyResetWindow:  time.Hour,
		VerifyResetLockFor: time.Hour,

		PATMaxPerUser: 20,
	}
}

// Service is the identity-service application layer.
type Service struct {
	store   repo.Store
	cfg     Config
	now     func() time.Time
	revoker RefreshRevoker // optional; nil before OIDC wiring
	mailer  mailer.Mailer  // optional; nil sends no mail (links still issued)
	patKey  []byte         // derived HMAC key for PAT hashing
}

func New(store repo.Store, cfg Config) *Service {
	return &Service{
		store:  store,
		cfg:    cfg,
		now:    time.Now,
		patKey: pat.DeriveKey(cfg.PATHMACSecret),
	}
}

// SetNow replaces the clock function used by the service. Used in tests to
// control time-dependent behaviour (expiry, throttle windows, etc.).
func (s *Service) SetNow(fn func() time.Time) { s.now = fn }

// SetRefreshRevoker wires OIDC refresh revocation into passive logout. Called in
// main after the OIDC store is built.
func (s *Service) SetRefreshRevoker(r RefreshRevoker) { s.revoker = r }

// SetMailer wires the transactional mailer used by email-verify / password-reset.
// Called in main after the mailer is built (mirrors SetRefreshRevoker).
func (s *Service) SetMailer(m mailer.Mailer) { s.mailer = m }
