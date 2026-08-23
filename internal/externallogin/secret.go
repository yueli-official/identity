package externallogin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

type SecretBox struct {
	gcm cipher.AEAD
}

func NewSecretBox(material string) (*SecretBox, error) {
	if len(material) < 32 {
		return nil, errors.New("external login secret material must contain at least 32 bytes")
	}
	key := sha256.Sum256([]byte(material))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{gcm: gcm}, nil
}

func (box *SecretBox) Encrypt(provider, plain string) (string, error) {
	nonce := make([]byte, box.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := box.gcm.Seal(nonce, nonce, []byte(plain), []byte(provider))
	return "v1:" + base64.StdEncoding.EncodeToString(sealed), nil
}

func (box *SecretBox) Decrypt(provider, encrypted string) (string, error) {
	raw := strings.TrimPrefix(encrypted, "v1:")
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	if len(data) < box.gcm.NonceSize() {
		return "", errors.New("encrypted external login secret is too short")
	}
	nonce := data[:box.gcm.NonceSize()]
	plain, err := box.gcm.Open(nil, nonce, data[box.gcm.NonceSize():], []byte(provider))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
