package publisher

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gowebpki/jcs"
)

type LocalKeyProvider struct {
	private   *ecdsa.PrivateKey
	keyID     string
	activated time.Time
}

func NewLocalKeyProvider() (*LocalKeyProvider, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return newLocalKeyProvider(private)
}

func newLocalKeyProvider(private *ecdsa.PrivateKey) (*LocalKeyProvider, error) {
	keyID, err := jwkThumbprint(&private.PublicKey)
	if err != nil {
		return nil, err
	}
	return &LocalKeyProvider{
		private: private, keyID: keyID, activated: time.Now().UTC(),
	}, nil
}

func LoadOrCreateLocalKey(path string) (*LocalKeyProvider, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("publisher local key file is required")
	}
	if raw, err := os.ReadFile(path); err == nil {
		return parseLocalPrivateKey(raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		return parseLocalPrivateKey(existing)
	}
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return newLocalKeyProvider(private)
}

func parseLocalPrivateKey(raw []byte) (*LocalKeyProvider, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("publisher local key file has no PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || private.Curve != elliptic.P256() {
		return nil, fmt.Errorf("publisher local key must be ECDSA P-256")
	}
	return newLocalKeyProvider(private)
}

func (provider *LocalKeyProvider) Sign(_ context.Context, data []byte) ([]byte, error) {
	if provider == nil || provider.private == nil {
		return nil, ErrSigningUnavailable
	}
	digest := sha256.Sum256(data)
	return ecdsa.SignASN1(rand.Reader, provider.private, digest[:])
}

func (provider *LocalKeyProvider) KeyID() (string, error) {
	if provider == nil || provider.keyID == "" {
		return "", ErrSigningUnavailable
	}
	return provider.keyID, nil
}

func (provider *LocalKeyProvider) VerificationKeys() []VerificationKey {
	if provider == nil || provider.private == nil {
		return nil
	}
	return []VerificationKey{{
		KeyID: provider.keyID, Algorithm: "ES256", Purpose: KeyPurpose, Status: KeyStatusActive,
		PublicJWK: publicJWK(&provider.private.PublicKey), ActivatedAt: provider.activated,
	}}
}

type ecdsaVerifier struct {
	key   *ecdsa.PublicKey
	keyID string
}

func (verifier ecdsaVerifier) Verify(_ context.Context, data, signature []byte) error {
	digest := sha256.Sum256(data)
	if !ecdsa.VerifyASN1(verifier.key, digest[:], signature) {
		return ErrInvalidAttestation
	}
	return nil
}

func (verifier ecdsaVerifier) KeyID() (string, error)   { return verifier.keyID, nil }
func (verifier ecdsaVerifier) Public() crypto.PublicKey { return verifier.key }

func publicJWK(public *ecdsa.PublicKey) map[string]any {
	size := (public.Curve.Params().BitSize + 7) / 8
	return map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(public.X.FillBytes(make([]byte, size))),
		"y":   base64.RawURLEncoding.EncodeToString(public.Y.FillBytes(make([]byte, size))),
	}
}

func jwkThumbprint(public *ecdsa.PublicKey) (string, error) {
	raw, err := canonicalJSON(publicJWK(public))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func verifierFromKey(key VerificationKey) (ecdsaVerifier, error) {
	if key.Purpose != KeyPurpose || (key.Status != "active" && key.Status != "retired") || key.Algorithm != "ES256" {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	if key.PublicJWK["kty"] != "EC" || key.PublicJWK["crv"] != "P-256" {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	xRaw, xOK := key.PublicJWK["x"].(string)
	yRaw, yOK := key.PublicJWK["y"].(string)
	if !xOK || !yOK {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	x, err := base64.RawURLEncoding.DecodeString(xRaw)
	if err != nil {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	y, err := base64.RawURLEncoding.DecodeString(yRaw)
	if err != nil {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	public, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append([]byte{4}, append(x, y...)...))
	if err != nil {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	calculated, err := jwkThumbprint(public)
	if err != nil || calculated != key.KeyID {
		return ecdsaVerifier{}, ErrInvalidAttestation
	}
	return ecdsaVerifier{key: public, keyID: key.KeyID}, nil
}

func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}
