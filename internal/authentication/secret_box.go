package authentication

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

const secretBoxVersion byte = 1

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(masterSecret []byte) (*SecretBox, error) {
	if len(masterSecret) < 32 {
		return nil, errors.New("authentication secret encryption master key must be at least 32 bytes")
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(
		sha256.New, masterSecret, nil, []byte("identity/totp-secret/v1"),
	), key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

func (box *SecretBox) Seal(plaintext, additionalData []byte) ([]byte, error) {
	nonce := make([]byte, box.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 1, 1+len(nonce)+len(plaintext)+box.aead.Overhead())
	out[0] = secretBoxVersion
	out = append(out, nonce...)
	out = box.aead.Seal(out, nonce, plaintext, additionalData)
	return out, nil
}

func (box *SecretBox) Open(ciphertext, additionalData []byte) ([]byte, error) {
	nonceSize := box.aead.NonceSize()
	if len(ciphertext) < 1+nonceSize+box.aead.Overhead() ||
		ciphertext[0] != secretBoxVersion {
		return nil, errors.New("unsupported or invalid encrypted authentication secret")
	}
	nonce := ciphertext[1 : 1+nonceSize]
	plaintext, err := box.aead.Open(nil, nonce, ciphertext[1+nonceSize:], additionalData)
	if err != nil {
		return nil, errors.New("encrypted authentication secret failed authentication")
	}
	return plaintext, nil
}
