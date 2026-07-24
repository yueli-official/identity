package logic

import (
	"context"
	"time"

	"github.com/yueli-official/foundation/go/abuse"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/identityabuse"
	"platform/services/identity/internal/mailer"
	identitypassword "platform/services/identity/internal/password"
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
	PasswordPolicy  identitypassword.Config

	// Email verification + password reset.
	AccountBaseURL     string        // base URL the action links point at
	VerifyTokenTTL     time.Duration // verify-email token lifetime
	ResetTokenTTL      time.Duration // password-reset token lifetime
	VerifyMaxReq       int           // max verify-email requests per window
	ResetMaxReq        int           // max password-reset requests per window
	VerifyResetWindow  time.Duration // throttle counting window (verify + reset)
	VerifyResetLockFor time.Duration // throttle lockout duration (verify + reset)

	// Personal Access Tokens (PAT).
	PATHMACSecret string // HKDF secret; empty → dev fallback (codec handles it)
	PATMaxPerUser int    // max PATs per identity (≤0 → clamped to 20 in New)
}

func DefaultConfig() Config {
	return Config{
		SessionIdleTTL:  30 * 24 * time.Hour,
		LoginMaxFails:   5,
		LoginFailWindow: 15 * time.Minute,
		LoginLockFor:    15 * time.Minute,
		IPMaxFails:      50,
		PasswordPolicy:  identitypassword.DefaultConfig(),

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
	store        repo.Store
	cfg          Config
	now          func() time.Time
	revoker      RefreshRevoker // optional; nil before OIDC wiring
	mailer       mailer.Mailer  // optional; nil sends no mail (links still issued)
	patKey       []byte         // derived HMAC key for PAT hashing
	abuse        identityabuse.Actions
	secondFactor SecondFactorGate
	passwords    *identitypassword.Manager
}

type SecondFactorGate interface {
	BeginSecondFactor(
		context.Context,
		string,
		authentication.Context,
		string,
		string,
	) (authentication.BeginSecondFactorResult, error)
}

func New(store repo.Store, cfg Config) *Service {
	// Defensive default: a zero/unset cap would make CreatePAT reject every token
	// (count >= 0 is always true). DefaultConfig sets 20; clamp bare configs too.
	if cfg.PATMaxPerUser <= 0 {
		cfg.PATMaxPerUser = 20
	}
	service := &Service{
		store:     store,
		cfg:       cfg,
		now:       time.Now,
		patKey:    pat.DeriveKey(cfg.PATHMACSecret),
		passwords: identitypassword.New(cfg.PasswordPolicy),
	}
	catalog := abuse.MustCompile(identityabuse.Definition(identityabuse.Policy{
		LoginAccountCapacity: int64(cfg.LoginMaxFails),
		LoginNetworkCapacity: int64(cfg.IPMaxFails),
		LoginWindow:          cfg.LoginFailWindow,
	}))
	module, err := abuse.NewMemory(catalog, abuse.MemoryOptions{
		Secret: []byte("identity-default-abuse-memory-secret"),
	})
	if err != nil {
		panic(err)
	}
	service.SetAbuseModule(module)
	return service
}

// SetRefreshRevoker wires OIDC refresh revocation into passive logout. Called in
// main after the OIDC store is built.
func (s *Service) SetRefreshRevoker(r RefreshRevoker) { s.revoker = r }

// SetMailer wires the transactional mailer used by email-verify / password-reset.
// Called in main after the mailer is built (mirrors SetRefreshRevoker).
func (s *Service) SetMailer(m mailer.Mailer) { s.mailer = m }

func (s *Service) SetSecondFactorGate(gate SecondFactorGate) {
	s.secondFactor = gate
}

// SetAbuseModule replaces the deterministic in-memory test default with the
// instance-local durable runtime assembled by main.
func (s *Service) SetAbuseModule(module abuse.Module) {
	actions, err := identityabuse.Bind(module)
	if err != nil {
		panic(err)
	}
	s.abuse = actions
}
