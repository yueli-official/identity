package publisher

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// CryptoSignerProvider adapts remote KMS/HSM implementations that expose the
// standard crypto.Signer contract. The private key is never available through
// this module; only the P-256 public key and signing operation are required.
type CryptoSignerProvider struct {
	signer crypto.Signer
	key    VerificationKey
}

func NewCryptoSignerProvider(
	signer crypto.Signer,
	activatedAt time.Time,
) (*CryptoSignerProvider, error) {
	if signer == nil {
		return nil, ErrSigningUnavailable
	}
	public, ok := signer.Public().(*ecdsa.PublicKey)
	if !ok || public.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%w: crypto.Signer must expose ECDSA P-256", ErrSigningUnavailable)
	}
	keyID, err := jwkThumbprint(public)
	if err != nil {
		return nil, err
	}
	if activatedAt.IsZero() {
		activatedAt = time.Now().UTC()
	}
	return &CryptoSignerProvider{
		signer: signer,
		key: VerificationKey{
			KeyID: keyID, Algorithm: "ES256", Purpose: KeyPurpose,
			Status: KeyStatusActive, PublicJWK: publicJWK(public),
			ActivatedAt: activatedAt.UTC(),
		},
	}, nil
}

// NewSecretPEMKeyProvider is the production secret-manager adapter. The PEM is
// injected into process memory and is never written to disk. KMS/HSM deployments
// should construct NewCryptoSignerProvider with their remote crypto.Signer.
func NewSecretPEMKeyProvider(value string) (*CryptoSignerProvider, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, fmt.Errorf("%w: publisher private key PEM is missing", ErrSigningUnavailable)
	}
	var parsed any
	var err error
	switch block.Type {
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningUnavailable, err)
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: publisher private key must be ECDSA P-256", ErrSigningUnavailable)
	}
	return NewCryptoSignerProvider(private, time.Time{})
}

func (provider *CryptoSignerProvider) Sign(
	_ context.Context,
	data []byte,
) ([]byte, error) {
	if provider == nil || provider.signer == nil {
		return nil, ErrSigningUnavailable
	}
	digest := sha256.Sum256(data)
	signature, err := provider.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningUnavailable, err)
	}
	return signature, nil
}

func (provider *CryptoSignerProvider) KeyID() (string, error) {
	if provider == nil || provider.key.KeyID == "" {
		return "", ErrSigningUnavailable
	}
	return provider.key.KeyID, nil
}

func (provider *CryptoSignerProvider) VerificationKeys() []VerificationKey {
	if provider == nil || provider.key.KeyID == "" {
		return nil
	}
	return []VerificationKey{cloneVerificationKey(provider.key)}
}

var _ KeyProvider = (*CryptoSignerProvider)(nil)
