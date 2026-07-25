package publisher

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

const (
	TrustManifestSchema = "https://yueli.dev/schemas/publisher-trust-manifest/v1"
	TrustRootPurpose    = "publisher-attestation-trust-root"
)

type TrustRoot struct {
	KeyID     string         `json:"keyId"`
	Algorithm string         `json:"algorithm"`
	Purpose   string         `json:"purpose"`
	PublicJWK map[string]any `json:"publicJwk"`
}

type TrustManifest struct {
	Schema          string            `json:"schema"`
	ManifestVersion uint64            `json:"manifestVersion"`
	Issuer          string            `json:"issuer"`
	IssuedAt        time.Time         `json:"issuedAt"`
	PolicyVersion   string            `json:"policyVersion"`
	RootKeyID       string            `json:"rootKeyId"`
	Keys            []VerificationKey `json:"keys"`
	Signature       string            `json:"manifestSignature"`
}

type VerifiedTrustManifest struct {
	Manifest     TrustManifest
	SnapshotHash string
}

type TrustManifestSigner interface {
	Sign(context.Context, []byte) ([]byte, error)
	KeyID() (string, error)
	TrustRoot() TrustRoot
}

type OfflineRoot struct {
	private *ecdsa.PrivateKey
	keyID   string
}

func NewOfflineRoot() (*OfflineRoot, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return newOfflineRoot(private)
}

func (root *OfflineRoot) Sign(_ context.Context, data []byte) ([]byte, error) {
	if root == nil || root.private == nil {
		return nil, ErrSigningUnavailable
	}
	digest := sha256.Sum256(data)
	return ecdsa.SignASN1(rand.Reader, root.private, digest[:])
}

func (root *OfflineRoot) KeyID() (string, error) {
	if root == nil || root.keyID == "" {
		return "", ErrSigningUnavailable
	}
	return root.keyID, nil
}

func (root *OfflineRoot) TrustRoot() TrustRoot {
	if root == nil || root.private == nil {
		return TrustRoot{}
	}
	return TrustRoot{
		KeyID: root.keyID, Algorithm: "ES256", Purpose: TrustRootPurpose,
		PublicJWK: publicJWK(&root.private.PublicKey),
	}
}

func SignTrustManifest(
	ctx context.Context,
	manifest TrustManifest,
	signer TrustManifestSigner,
) (TrustManifest, error) {
	if signer == nil {
		return TrustManifest{}, ErrSigningUnavailable
	}
	keyID, err := signer.KeyID()
	if err != nil {
		return TrustManifest{}, ErrSigningUnavailable
	}
	manifest.RootKeyID = keyID
	manifest.Signature = ""
	if err := validateTrustManifest(manifest, false); err != nil {
		return TrustManifest{}, err
	}
	payload, err := trustManifestPayload(manifest)
	if err != nil {
		return TrustManifest{}, invalidTrust(err)
	}
	signature, err := signer.Sign(ctx, payload)
	if err != nil {
		return TrustManifest{}, fmt.Errorf("%w: %v", ErrSigningUnavailable, err)
	}
	manifest.Signature = base64.RawURLEncoding.EncodeToString(signature)
	return manifest, nil
}

func VerifyTrustManifest(
	ctx context.Context,
	manifest TrustManifest,
	roots []TrustRoot,
) (VerifiedTrustManifest, error) {
	if err := validateTrustManifest(manifest, true); err != nil {
		return VerifiedTrustManifest{}, err
	}
	var trusted TrustRoot
	for _, root := range roots {
		if root.KeyID == manifest.RootKeyID {
			trusted = root
			break
		}
	}
	if trusted.KeyID == "" {
		return VerifiedTrustManifest{}, ErrUntrustedRoot
	}
	public, err := validateTrustRoot(trusted)
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	payload, err := trustManifestPayload(manifest)
	if err != nil {
		return VerifiedTrustManifest{}, invalidTrust(err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(manifest.Signature)
	if err != nil {
		return VerifiedTrustManifest{}, invalidTrust(err)
	}
	digest := sha256.Sum256(payload)
	if !ecdsa.VerifyASN1(public, digest[:], signature) {
		return VerifiedTrustManifest{}, ErrInvalidTrustManifest
	}
	snapshot, err := canonicalJSON(manifest)
	if err != nil {
		return VerifiedTrustManifest{}, invalidTrust(err)
	}
	return VerifiedTrustManifest{
		Manifest: manifest, SnapshotHash: "sha256:" + digestHex(snapshot),
	}, nil
}

func LoadTrustManifest(
	raw []byte,
	roots []TrustRoot,
	expectedIssuer string,
	expectedActiveKeyID string,
	minimumVersion uint64,
) (VerifiedTrustManifest, error) {
	var manifest TrustManifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return VerifiedTrustManifest{}, invalidTrust(err)
	}
	verified, err := VerifyTrustManifest(context.Background(), manifest, roots)
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	if manifest.Issuer != strings.TrimSpace(expectedIssuer) {
		return VerifiedTrustManifest{}, ErrInvalidTrustManifest
	}
	if minimumVersion == 0 || manifest.ManifestVersion < minimumVersion {
		return VerifiedTrustManifest{}, ErrInvalidTrustManifest
	}
	if expectedActiveKeyID == "" {
		for _, key := range manifest.Keys {
			if key.Status == KeyStatusActive {
				return VerifiedTrustManifest{}, ErrInvalidTrustManifest
			}
		}
		return verified, nil
	}
	for _, key := range manifest.Keys {
		if key.KeyID == expectedActiveKeyID && key.Status == KeyStatusActive {
			return verified, nil
		}
	}
	return VerifiedTrustManifest{}, ErrInvalidTrustManifest
}

func TrustManifestResource(raw []byte) (string, error) {
	var manifest TrustManifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return "", invalidTrust(err)
	}
	canonical, err := canonicalJSON(manifest)
	if err != nil {
		return "", invalidTrust(err)
	}
	return "publisher:trust-manifest:sha256:" + digestHex(canonical), nil
}

func trustManifestPayload(manifest TrustManifest) ([]byte, error) {
	manifest.Signature = ""
	type unsignedManifest struct {
		Schema          string            `json:"schema"`
		ManifestVersion uint64            `json:"manifestVersion"`
		Issuer          string            `json:"issuer"`
		IssuedAt        time.Time         `json:"issuedAt"`
		PolicyVersion   string            `json:"policyVersion"`
		RootKeyID       string            `json:"rootKeyId"`
		Keys            []VerificationKey `json:"keys"`
	}
	return canonicalJSON(unsignedManifest{
		Schema: manifest.Schema, ManifestVersion: manifest.ManifestVersion,
		Issuer: manifest.Issuer, IssuedAt: manifest.IssuedAt,
		PolicyVersion: manifest.PolicyVersion, RootKeyID: manifest.RootKeyID,
		Keys: manifest.Keys,
	})
}

func validateTrustManifest(manifest TrustManifest, requireSignature bool) error {
	if manifest.Schema != TrustManifestSchema ||
		manifest.ManifestVersion == 0 ||
		!validStableURI(manifest.Issuer) ||
		manifest.IssuedAt.IsZero() ||
		strings.TrimSpace(manifest.PolicyVersion) == "" ||
		strings.TrimSpace(manifest.RootKeyID) == "" ||
		len(manifest.Keys) == 0 ||
		len(manifest.Keys) > 32 ||
		(requireSignature && strings.TrimSpace(manifest.Signature) == "") {
		return ErrInvalidTrustManifest
	}
	seen := make(map[string]struct{}, len(manifest.Keys))
	active := 0
	for _, key := range manifest.Keys {
		if _, duplicate := seen[key.KeyID]; duplicate {
			return ErrInvalidTrustManifest
		}
		seen[key.KeyID] = struct{}{}
		if err := validateManifestKey(key); err != nil {
			return err
		}
		if key.Status == KeyStatusActive {
			active++
		}
	}
	if active > 1 {
		return ErrInvalidTrustManifest
	}
	return nil
}

func validateManifestKey(key VerificationKey) error {
	switch key.Status {
	case KeyStatusPreactive, KeyStatusActive:
		if key.RetiredAt != nil || key.CompromisedAt != nil || key.RevokedAt != nil {
			return ErrInvalidTrustManifest
		}
	case KeyStatusRetired:
		if key.RetiredAt == nil {
			return ErrInvalidTrustManifest
		}
	case KeyStatusCompromised:
		if key.CompromisedAt == nil {
			return ErrInvalidTrustManifest
		}
	case KeyStatusRevoked:
		if key.RevokedAt == nil || strings.TrimSpace(key.RevocationReason) == "" {
			return ErrInvalidTrustManifest
		}
	default:
		return ErrInvalidTrustManifest
	}
	if key.KeyID == "" || key.Algorithm != "ES256" || key.Purpose != KeyPurpose ||
		key.ActivatedAt.IsZero() {
		return ErrInvalidTrustManifest
	}
	candidate := key
	candidate.Status = KeyStatusActive
	if _, err := verifierFromKey(candidate); err != nil {
		return invalidTrust(err)
	}
	return nil
}

func validateTrustRoot(root TrustRoot) (*ecdsa.PublicKey, error) {
	if root.KeyID == "" || root.Algorithm != "ES256" || root.Purpose != TrustRootPurpose {
		return nil, ErrUntrustedRoot
	}
	candidate := VerificationKey{
		KeyID: root.KeyID, Algorithm: root.Algorithm, Purpose: KeyPurpose,
		Status: KeyStatusActive, PublicJWK: root.PublicJWK,
	}
	verifier, err := verifierFromKey(candidate)
	if err != nil {
		return nil, ErrUntrustedRoot
	}
	return verifier.key, nil
}

func invalidTrust(err error) error {
	if err == nil {
		return ErrInvalidTrustManifest
	}
	return fmt.Errorf("%w: %v", ErrInvalidTrustManifest, err)
}
