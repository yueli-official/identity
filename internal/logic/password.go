package logic

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12
const minPasswordLen = 8

// dummyBcryptHash equalizes login timing on the unknown-email path to mitigate
// account enumeration. Computed once at package init.
var dummyBcryptHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalization-placeholder"), bcryptCost)

// VerifyDummy runs a bcrypt comparison against a constant hash purely to spend
// the same time as a real verification (timing equalization). Result is ignored.
func VerifyDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(plain))
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func ValidatePasswordStrength(plain string) error {
	if len(plain) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}
