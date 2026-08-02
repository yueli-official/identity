// Package user owns the stable rules that distinguish internal user IDs,
// public user keys and human-readable handles. Transport and storage adapters
// consume these types instead of reproducing identifier policy.
package user

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	publicKeyPrefix = "usr_"
	publicKeyBytes  = 16
)

var (
	publicKeyPattern = regexp.MustCompile(`^usr_[A-Za-z0-9_-]{22}$`)
	handlePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$`)
	reservedHandles  = map[string]struct{}{
		"about": {}, "account": {}, "admin": {}, "api": {}, "assets": {},
		"auth": {}, "blog": {}, "callback": {}, "cdn": {}, "docs": {},
		"gallery": {}, "help": {}, "home": {}, "identity": {}, "login": {},
		"logout": {}, "media": {}, "oauth": {}, "oidc": {}, "privacy": {},
		"profile": {}, "register": {}, "resource": {}, "security": {},
		"settings": {}, "shop": {}, "status": {}, "support": {}, "system": {},
		"terms": {}, "user": {}, "users": {}, "www": {},
	}
)

// PublicKey is the stable opaque identifier exposed by public user contracts.
type PublicKey string

func NewPublicKey() (PublicKey, error) {
	value, err := newOpaqueID(publicKeyPrefix)
	return PublicKey(value), err
}

func NewPairwiseSubject() (string, error) {
	return newOpaqueID("psu_")
}

func newOpaqueID(prefix string) (string, error) {
	raw := make([]byte, publicKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate opaque user identifier: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func ParsePublicKey(value string) (PublicKey, error) {
	if !publicKeyPattern.MatchString(value) {
		return "", errors.New("invalid public user key")
	}
	return PublicKey(value), nil
}

// Handle is a canonical, mutable, human-readable alias. It is never an
// ownership key and retired values remain reserved by the persistence layer.
type Handle string

func NormalizeHandle(value string) (Handle, error) {
	canonical := strings.ToLower(strings.TrimSpace(value))
	if !handlePattern.MatchString(canonical) {
		return "", errors.New("handle must be 3-30 lowercase ASCII letters, digits, or underscores and start/end with alphanumeric")
	}
	if _, reserved := reservedHandles[canonical]; reserved {
		return "", errors.New("handle is reserved")
	}
	return Handle(canonical), nil
}

func NormalizeOptionalHandle(value string) (Handle, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return NormalizeHandle(value)
}
