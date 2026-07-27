package guest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/oidc"
	"github.com/yueli-official/identity/internal/repo"
)

var (
	ErrInvalidRequest  = errors.New("invalid guest session request")
	ErrInvalidSession  = errors.New("invalid guest session")
	ErrInvalidAudience = errors.New("invalid guest token audience")
	ErrClaimConflict   = errors.New("guest session claim conflict")
)

type Config struct {
	Issuer         string
	MaxSessionTTL  time.Duration
	AccessTokenTTL time.Duration
	Now            func() time.Time
}

type Service struct {
	store   repo.GuestSessionStore
	clients repo.ClientRepo
	keys    *oidc.Manager
	cfg     Config
}

type Created struct {
	SubjectID    string
	SessionToken string
	EffectiveTTL time.Duration
	ExpiresAt    time.Time
}

type Issued struct {
	AccessToken string
	ExpiresIn   time.Duration
}

type Claim struct {
	SubjectID  string
	UserID     string
	ClaimedAt  time.Time
	ClaimToken string
}

func New(store repo.GuestSessionStore, clients repo.ClientRepo, keys *oidc.Manager, cfg Config) *Service {
	if cfg.MaxSessionTTL <= 0 {
		cfg.MaxSessionTTL = 30 * 24 * time.Hour
	}
	if cfg.AccessTokenTTL <= 0 {
		cfg.AccessTokenTTL = 10 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{store: store, clients: clients, keys: keys, cfg: cfg}
}

func (s *Service) Create(ctx context.Context, clientID string, requestedTTL time.Duration) (Created, error) {
	clientID = strings.TrimSpace(clientID)
	if requestedTTL <= 0 || clientID == "" {
		return Created{}, ErrInvalidRequest
	}
	if _, err := s.clients.GetClient(ctx, clientID); err != nil {
		return Created{}, ErrInvalidRequest
	}
	effective := min(requestedTTL, s.cfg.MaxSessionTTL)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Created{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.cfg.Now().UTC()
	session := model.GuestSession{
		ID: uuid.NewString(), TokenHash: tokenHash(token), ClientID: clientID,
		CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(effective),
	}
	if err := s.store.CreateGuestSession(ctx, session); err != nil {
		return Created{}, err
	}
	return Created{SubjectID: session.ID, SessionToken: token, EffectiveTTL: effective, ExpiresAt: session.ExpiresAt}, nil
}

func (s *Service) Token(ctx context.Context, clientID, sessionToken, audience string) (Issued, error) {
	client, err := s.clients.GetClient(ctx, strings.TrimSpace(clientID))
	if err != nil || !slices.Contains(client.Audiences, audience) {
		return Issued{}, ErrInvalidAudience
	}
	session, err := s.active(ctx, client.ID, sessionToken)
	if err != nil {
		return Issued{}, err
	}
	raw, err := s.keys.MintGuestToken(s.cfg.Issuer, session.ID, client.ID, audience, s.cfg.AccessTokenTTL, s.cfg.Now().UTC())
	if err != nil {
		return Issued{}, err
	}
	return Issued{AccessToken: raw, ExpiresIn: s.cfg.AccessTokenTTL}, nil
}

func (s *Service) Claim(ctx context.Context, clientID, sessionToken, identityID string) (Claim, error) {
	identityID = strings.TrimSpace(identityID)
	if _, err := uuid.Parse(identityID); err != nil {
		return Claim{}, ErrInvalidRequest
	}
	session, err := s.session(ctx, strings.TrimSpace(clientID), sessionToken, true)
	if err != nil {
		return Claim{}, err
	}
	claimed, err := s.store.ClaimGuestSession(ctx, tokenHash(sessionToken), identityID, s.cfg.Now().UTC())
	if errors.Is(err, repo.ErrGuestClaimConflict) {
		return Claim{}, ErrClaimConflict
	}
	if err != nil {
		return Claim{}, err
	}
	return Claim{SubjectID: session.ID, UserID: claimed.ClaimedIdentityID, ClaimedAt: *claimed.ClaimedAt}, nil
}

func (s *Service) ClaimForAudience(ctx context.Context, clientID, sessionToken, identityID, audience string) (Claim, error) {
	client, err := s.clients.GetClient(ctx, strings.TrimSpace(clientID))
	if err != nil || !slices.Contains(client.Audiences, strings.TrimSpace(audience)) {
		return Claim{}, ErrInvalidAudience
	}
	claimed, err := s.Claim(ctx, client.ID, sessionToken, identityID)
	if err != nil {
		return Claim{}, err
	}
	claimed.ClaimToken, err = s.keys.MintGuestClaimToken(
		s.cfg.Issuer, claimed.UserID, claimed.SubjectID, client.ID, audience, s.cfg.AccessTokenTTL, s.cfg.Now().UTC(),
	)
	return claimed, err
}

func (s *Service) active(ctx context.Context, clientID, sessionToken string) (model.GuestSession, error) {
	return s.session(ctx, clientID, sessionToken, false)
}

func (s *Service) session(ctx context.Context, clientID, sessionToken string, allowClaimed bool) (model.GuestSession, error) {
	if clientID == "" || sessionToken == "" {
		return model.GuestSession{}, ErrInvalidSession
	}
	session, err := s.store.GetGuestSession(ctx, tokenHash(sessionToken))
	if err != nil || session.ClientID != clientID || session.RevokedAt != nil || (!allowClaimed && session.ClaimedIdentityID != "") || !session.ExpiresAt.After(s.cfg.Now()) {
		return model.GuestSession{}, ErrInvalidSession
	}
	return session, nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
