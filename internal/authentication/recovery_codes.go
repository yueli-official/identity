package authentication

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	recoveryCodeCount = 10
	recoveryCodeBytes = 10
)

type RecoveryCodeCodec struct {
	key []byte
}

func NewRecoveryCodeCodec(masterSecret []byte) (*RecoveryCodeCodec, error) {
	if len(masterSecret) < 32 {
		return nil, errors.New("recovery code master key must be at least 32 bytes")
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(
		sha256.New, masterSecret, nil, []byte("identity/recovery-code/v1"),
	), key); err != nil {
		return nil, err
	}
	return &RecoveryCodeCodec{key: key}, nil
}

func (codec *RecoveryCodeCodec) Generate() ([]string, [][]byte, error) {
	codes := make([]string, recoveryCodeCount)
	digests := make([][]byte, recoveryCodeCount)
	seen := map[string]struct{}{}
	for index := 0; index < recoveryCodeCount; {
		raw := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		canonical := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		codes[index] = strings.Join([]string{
			canonical[0:4], canonical[4:8], canonical[8:12], canonical[12:16],
		}, "-")
		digests[index] = codec.Digest(canonical)
		index++
	}
	return codes, digests, nil
}

func (codec *RecoveryCodeCodec) Digest(code string) []byte {
	canonical := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(code))
	mac := hmac.New(sha256.New, codec.key)
	_, _ = mac.Write([]byte(canonical))
	return mac.Sum(nil)
}
