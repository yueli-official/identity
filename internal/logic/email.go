// Package logic holds identity-service business logic. It depends only on
// repo interfaces (no DB/Redis), making it fully unit-testable with fakes.
package logic

import (
	"errors"
	"regexp"
	"strings"
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// CanonicalizeEmail lowercases and trims; the canonical form is what we store
// and uniqueness-check on.
func CanonicalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateEmail(email string) error {
	if !emailRE.MatchString(CanonicalizeEmail(email)) {
		return errors.New("invalid email format")
	}
	return nil
}
