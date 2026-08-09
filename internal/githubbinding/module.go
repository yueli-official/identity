package githubbinding

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yueli-official/foundation/go/identifier"
)

const defaultAttemptTTL = 10 * time.Minute

type Module struct {
	store                   Store
	provider                Provider
	aead                    cipher.AEAD
	ttl                     time.Duration
	now                     func() time.Time
	random                  io.Reader
	resolvePublisherSubject func(context.Context, string) (string, error)
}

func New(config Config) (*Module, error) {
	if config.Store == nil || config.Provider == nil || config.ResolvePublisherSubject == nil || len(config.CipherSecret) < 32 {
		return nil, ErrUnavailable
	}
	key := sha256.Sum256(append([]byte("identity/github-binding/pkce/v1\x00"), config.CipherSecret...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	ttl := config.AttemptTTL
	if ttl <= 0 {
		ttl = defaultAttemptTTL
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Module{
		store: config.Store, provider: config.Provider, aead: aead,
		ttl: ttl, now: now, random: rand.Reader,
		resolvePublisherSubject: config.ResolvePublisherSubject,
	}, nil
}

func (module *Module) Begin(
	ctx context.Context,
	identityID string,
	sessionID string,
	returnTo string,
) (BeginResult, error) {
	if strings.TrimSpace(identityID) == "" || strings.TrimSpace(sessionID) == "" {
		return BeginResult{}, ErrInvalidAttempt
	}
	state, err := randomToken(module.random, 32)
	if err != nil {
		return BeginResult{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	verifier, err := randomToken(module.random, 32)
	if err != nil {
		return BeginResult{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	stateDigest := digest(state)
	encrypted, err := module.seal([]byte(verifier), []byte(stateDigest))
	if err != nil {
		return BeginResult{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	now := module.now().UTC()
	expiresAt := now.Add(module.ttl)
	attempt := Attempt{
		ID: identifier.MustNew().String(), StateDigest: stateDigest, IdentityID: identityID,
		SessionDigest: digest(sessionID), VerifierCiphertext: encrypted,
		ReturnTo: returnTo, ExpiresAt: expiresAt, CreatedAt: now,
	}
	if err := module.store.CreateAttempt(ctx, attempt); err != nil {
		return BeginResult{}, err
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	return BeginResult{
		AuthorizationURL: module.provider.AuthorizationURL(state, challenge),
		ExpiresAt:        expiresAt,
	}, nil
}

func (module *Module) Complete(
	ctx context.Context,
	state string,
	sessionID string,
	code string,
) (CompleteResult, error) {
	if state == "" || sessionID == "" || code == "" {
		return CompleteResult{}, ErrInvalidAttempt
	}
	stateDigest := digest(state)
	attempt, err := module.store.ConsumeAttempt(
		ctx, stateDigest, digest(sessionID), module.now().UTC(),
	)
	if err != nil {
		return CompleteResult{}, err
	}
	verifier, err := module.open(attempt.VerifierCiphertext, []byte(stateDigest))
	if err != nil {
		return CompleteResult{}, ErrInvalidAttempt
	}
	token, err := module.provider.ExchangeCode(ctx, code, string(verifier))
	if err != nil || token == "" {
		return CompleteResult{}, fmt.Errorf("%w: exchange", ErrProviderFailure)
	}
	defer func() { _ = module.provider.RevokeAccessToken(context.WithoutCancel(ctx), token) }()
	account, err := module.provider.AuthenticatedUser(ctx, token)
	if err != nil || account.AccountID == "" || strings.TrimSpace(account.Login) == "" {
		return CompleteResult{}, fmt.Errorf("%w: authenticated user", ErrProviderFailure)
	}
	bound, err := module.store.Bind(ctx, attempt.IdentityID, account, module.now().UTC())
	if err != nil {
		return CompleteResult{}, err
	}
	return CompleteResult{
		Binding: bound.Binding, ReturnTo: attempt.ReturnTo,
		Created: bound.Created, Renamed: bound.Renamed,
	}, nil
}

func (module *Module) List(ctx context.Context, identityID string) ([]Binding, error) {
	return module.store.ListByIdentity(ctx, identityID)
}

func (module *Module) Unbind(
	ctx context.Context,
	identityID string,
	bindingID string,
) (Binding, error) {
	return module.store.Unbind(ctx, identityID, bindingID, module.now().UTC())
}

func (module *Module) AuthorizationRevoked(
	ctx context.Context,
	accountID string,
	login string,
) ([]Binding, error) {
	if accountID == "" {
		return nil, ErrProviderFailure
	}
	return module.store.BlockByAccount(ctx, accountID, login, module.now().UTC())
}

func (module *Module) seal(plaintext, additional []byte) (string, error) {
	nonce := make([]byte, module.aead.NonceSize())
	if _, err := io.ReadFull(module.random, nonce); err != nil {
		return "", err
	}
	value := module.aead.Seal(nonce, nonce, plaintext, additional)
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (module *Module) open(encoded string, additional []byte) ([]byte, error) {
	value, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(value) < module.aead.NonceSize() {
		return nil, ErrInvalidAttempt
	}
	nonce := value[:module.aead.NonceSize()]
	return module.aead.Open(nil, nonce, value[module.aead.NonceSize():], additional)
}

func randomToken(reader io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
