// Package stepup verifies and atomically consumes Identity action-bound
// step-up proofs. A proof is authentication evidence, never authorization:
// consumers must still apply their own RBAC and domain policies.
package stepup

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	foundationauth "github.com/yueli-official/foundation/go/auth"
)

var (
	ErrInvalidProof = errors.New("step-up proof invalid")
	ErrReplay       = errors.New("step-up proof already consumed")
)

type ReplayStore interface {
	Consume(context.Context, string, time.Time) (bool, error)
}

type Config struct {
	Keys        foundationauth.KeySource
	Issuer      string
	Audience    string
	Replay      ReplayStore
	Clock       func() time.Time
	MaxLifetime time.Duration
}

type Verifier struct {
	tokens *foundationauth.Verifier
	replay ReplayStore
}

type Evidence struct {
	Subject      string
	SessionID    string
	Action       string
	ResourceHash string
	AuthTime     time.Time
	Methods      []string
	Profile      string
	ExpiresAt    time.Time
	JTI          string
}

func New(config Config) (*Verifier, error) {
	if config.Replay == nil {
		return nil, errors.New("stepup: Replay store is required")
	}
	maxLifetime := config.MaxLifetime
	if maxLifetime <= 0 {
		maxLifetime = 2 * time.Minute
	}
	tokens, err := foundationauth.NewVerifier(foundationauth.Config{
		Keys: config.Keys, Issuer: config.Issuer,
		Audiences:  []string{strings.TrimSpace(config.Audience)},
		Algorithms: []jose.SignatureAlgorithm{jose.RS256},
		Types:      []string{"step-up+jwt"}, MaxLifetime: maxLifetime,
		Clock: config.Clock,
	})
	if err != nil {
		return nil, err
	}
	return &Verifier{tokens: tokens, replay: config.Replay}, nil
}

func (verifier *Verifier) VerifyAndConsume(
	ctx context.Context,
	raw, expectedAction, resource string,
) (Evidence, error) {
	principal, err := verifier.tokens.Verify(ctx, raw)
	if err != nil {
		return Evidence{}, fmt.Errorf("%w: %v", ErrInvalidProof, err)
	}
	if !principal.IsUser() {
		return Evidence{}, ErrInvalidProof
	}
	tokenUse, _ := stringClaim(principal, "token_use")
	jti, _ := stringClaim(principal, "jti")
	sessionID, _ := stringClaim(principal, "sid")
	action, _ := stringClaim(principal, "action")
	resourceHash, _ := stringClaim(principal, "resource_hash")
	profile, _ := stringClaim(principal, "acr")
	recovery, _ := boolClaim(principal, "recovery")
	authTime, authTimeOK := unixClaim(principal, "auth_time")
	methods, methodsOK := stringsClaim(principal, "amr")
	expectedDigest := sha256.Sum256([]byte(resource))
	expectedHash := base64.RawURLEncoding.EncodeToString(expectedDigest[:])
	if tokenUse != "step_up" || jti == "" || sessionID == "" ||
		action != expectedAction || recovery || !authTimeOK || !methodsOK ||
		subtle.ConstantTimeCompare([]byte(resourceHash), []byte(expectedHash)) != 1 {
		return Evidence{}, ErrInvalidProof
	}
	consumed, err := verifier.replay.Consume(ctx, jti, principal.ExpiresAt)
	if err != nil {
		return Evidence{}, err
	}
	if !consumed {
		return Evidence{}, ErrReplay
	}
	return Evidence{
		Subject: principal.Subject, SessionID: sessionID, Action: action,
		ResourceHash: resourceHash, AuthTime: authTime, Methods: methods,
		Profile: profile, ExpiresAt: principal.ExpiresAt, JTI: jti,
	}, nil
}

func stringClaim(principal *foundationauth.Principal, key string) (string, bool) {
	value, ok := principal.Claim(key)
	text, valid := value.(string)
	return text, ok && valid && strings.TrimSpace(text) != ""
}

func boolClaim(principal *foundationauth.Principal, key string) (bool, bool) {
	value, ok := principal.Claim(key)
	result, valid := value.(bool)
	return result, ok && valid
}

func unixClaim(principal *foundationauth.Principal, key string) (time.Time, bool) {
	value, ok := principal.Claim(key)
	number, valid := value.(float64)
	if !ok || !valid || number <= 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(number), 0).UTC(), true
}

func stringsClaim(principal *foundationauth.Principal, key string) ([]string, bool) {
	value, ok := principal.Claim(key)
	items, valid := value.([]any)
	if !ok || !valid || len(items) == 0 {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, valid := item.(string)
		if !valid || strings.TrimSpace(text) == "" {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}
