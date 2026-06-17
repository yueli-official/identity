package logic

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12
const minPasswordLen = 8

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
