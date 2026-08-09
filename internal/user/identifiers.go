// Package user owns the stable rules that distinguish internal user IDs,
// public user keys and human-readable handles. Transport and storage adapters
// consume these types instead of reproducing identifier policy.
package user

import (
	"errors"
	"regexp"
	"strings"

	"github.com/yueli-official/foundation/go/identifier"
)

var (
	handlePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{1,28}[a-z0-9]$`)
	reservedHandles = map[string]struct{}{
		"about": {}, "account": {}, "admin": {}, "api": {}, "assets": {},
		"auth": {}, "blog": {}, "callback": {}, "cdn": {}, "docs": {},
		"gallery": {}, "help": {}, "home": {}, "identity": {}, "login": {},
		"logout": {}, "media": {}, "oauth": {}, "oidc": {}, "privacy": {},
		"profile": {}, "register": {}, "resource": {}, "security": {},
		"settings": {}, "shop": {}, "status": {}, "support": {}, "system": {},
		"terms": {}, "user": {}, "users": {}, "www": {},
	}
)

// PublicKey is the immutable eight-character public locator used by OIDC,
// cross-site references and the permanent account URL. It is not a credential.
type PublicKey string

func NewPublicKey() (PublicKey, error) {
	value, err := identifier.CompactURLV1.New()
	return PublicKey(value), err
}

func NewPairwiseSubject() (string, error) {
	value, err := identifier.OpaquePublicV1.New()
	return value.String(), err
}

func ParsePublicKey(value string) (PublicKey, error) {
	parsed, err := identifier.CompactURLV1.Parse(value)
	if err != nil {
		return "", errors.New("invalid public user key")
	}
	return PublicKey(parsed), nil
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
