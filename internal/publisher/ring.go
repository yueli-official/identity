package publisher

import (
	"context"
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
	"sync"
	"time"
)

const localKeyRingSchema = "https://yueli.dev/schemas/publisher-local-key-ring/v1"

type ManagedKeyProvider interface {
	KeyProvider
	PrepareRotation(context.Context) (VerificationKey, error)
	ApplyTrustManifest(context.Context, VerifiedTrustManifest) error
}

type localKeyRecord struct {
	Descriptor VerificationKey `json:"descriptor"`
	PrivateKey string          `json:"privateKeyPkcs8"`
}

type localKeyRingFile struct {
	Schema          string           `json:"schema"`
	ManifestVersion uint64           `json:"manifestVersion"`
	Keys            []localKeyRecord `json:"keys"`
}

type LocalKeyRing struct {
	mu              sync.RWMutex
	path            string
	manifestVersion uint64
	keys            []localKeyRecord
	private         map[string]*ecdsa.PrivateKey
}

func LoadOrCreateLocalKeyRing(path string) (*LocalKeyRing, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("publisher local key ring file is required")
	}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if block, _ := pem.Decode(raw); block != nil {
			legacy, parseErr := parseLocalPrivateKey(raw)
			if parseErr != nil {
				return nil, parseErr
			}
			ring, buildErr := newLocalKeyRing(path, 0, []localKeyRecord{{
				Descriptor: legacy.VerificationKeys()[0],
				PrivateKey: encodePrivateKey(legacy.private),
			}})
			if buildErr != nil {
				return nil, buildErr
			}
			if persistErr := ring.persist(ring.manifestVersion, ring.keys); persistErr != nil {
				return nil, persistErr
			}
			return ring, nil
		}
		var stored localKeyRingFile
		if decodeErr := decodeStrict(raw, &stored); decodeErr != nil {
			return nil, decodeErr
		}
		if stored.Schema != localKeyRingSchema {
			return nil, fmt.Errorf("publisher local key ring schema is invalid")
		}
		return newLocalKeyRing(path, stored.ManifestVersion, stored.Keys)
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	provider, err := newLocalKeyProvider(private)
	if err != nil {
		return nil, err
	}
	ring, err := newLocalKeyRing(path, 0, []localKeyRecord{{
		Descriptor: provider.VerificationKeys()[0],
		PrivateKey: encodePrivateKey(private),
	}})
	if err != nil {
		return nil, err
	}
	if err := ring.persist(0, ring.keys); err != nil {
		return nil, err
	}
	return ring, nil
}

func newLocalKeyRing(
	path string,
	manifestVersion uint64,
	records []localKeyRecord,
) (*LocalKeyRing, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("publisher local key ring has no keys")
	}
	ring := &LocalKeyRing{
		path: path, manifestVersion: manifestVersion, keys: cloneLocalRecords(records),
		private: make(map[string]*ecdsa.PrivateKey, len(records)),
	}
	active := 0
	for _, record := range records {
		private, err := decodePrivateKey(record.PrivateKey)
		if err != nil {
			return nil, err
		}
		keyID, err := jwkThumbprint(&private.PublicKey)
		if err != nil || keyID != record.Descriptor.KeyID {
			return nil, fmt.Errorf("publisher local key ring public/private key mismatch")
		}
		if err := validateStoredKey(record.Descriptor); err != nil {
			return nil, err
		}
		if record.Descriptor.Status == KeyStatusActive {
			active++
		}
		ring.private[keyID] = private
	}
	if active > 1 {
		return nil, ErrInvalidKeyTransition
	}
	return ring, nil
}

func (ring *LocalKeyRing) Sign(_ context.Context, data []byte) ([]byte, error) {
	ring.mu.RLock()
	defer ring.mu.RUnlock()
	private := ring.activePrivate()
	if private == nil {
		return nil, ErrSigningUnavailable
	}
	digest := sha256.Sum256(data)
	return ecdsa.SignASN1(rand.Reader, private, digest[:])
}

func (ring *LocalKeyRing) KeyID() (string, error) {
	ring.mu.RLock()
	defer ring.mu.RUnlock()
	for _, record := range ring.keys {
		if record.Descriptor.Status == KeyStatusActive {
			return record.Descriptor.KeyID, nil
		}
	}
	return "", ErrSigningUnavailable
}

func (ring *LocalKeyRing) VerificationKeys() []VerificationKey {
	ring.mu.RLock()
	defer ring.mu.RUnlock()
	result := make([]VerificationKey, len(ring.keys))
	for index := range ring.keys {
		result[index] = cloneVerificationKey(ring.keys[index].Descriptor)
	}
	return result
}

func (ring *LocalKeyRing) PrepareRotation(
	_ context.Context,
) (VerificationKey, error) {
	ring.mu.Lock()
	defer ring.mu.Unlock()
	for _, record := range ring.keys {
		if record.Descriptor.Status == KeyStatusPreactive {
			return VerificationKey{}, ErrRotationPending
		}
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return VerificationKey{}, err
	}
	provider, err := newLocalKeyProvider(private)
	if err != nil {
		return VerificationKey{}, err
	}
	descriptor := provider.VerificationKeys()[0]
	descriptor.Status = KeyStatusPreactive
	candidate := append(cloneLocalRecords(ring.keys), localKeyRecord{
		Descriptor: descriptor, PrivateKey: encodePrivateKey(private),
	})
	if err := ring.persist(ring.manifestVersion, candidate); err != nil {
		return VerificationKey{}, err
	}
	ring.keys = candidate
	ring.private[descriptor.KeyID] = private
	return cloneVerificationKey(descriptor), nil
}

func (ring *LocalKeyRing) ApplyTrustManifest(
	_ context.Context,
	verified VerifiedTrustManifest,
) error {
	ring.mu.Lock()
	defer ring.mu.Unlock()
	manifest := verified.Manifest
	if manifest.ManifestVersion < ring.manifestVersion ||
		len(manifest.Keys) != len(ring.keys) {
		return ErrInvalidKeyTransition
	}
	targets := make(map[string]VerificationKey, len(manifest.Keys))
	for _, key := range manifest.Keys {
		targets[key.KeyID] = key
	}
	candidate := cloneLocalRecords(ring.keys)
	for index, record := range ring.keys {
		target, ok := targets[record.Descriptor.KeyID]
		if !ok || !samePublicKey(record.Descriptor, target) ||
			!allowedKeyTransition(record.Descriptor.Status, target.Status) {
			return ErrInvalidKeyTransition
		}
		candidate[index].Descriptor = cloneVerificationKey(target)
	}
	if manifest.ManifestVersion == ring.manifestVersion {
		for index := range candidate {
			if !sameKeyDescriptor(candidate[index].Descriptor, ring.keys[index].Descriptor) {
				return ErrInvalidKeyTransition
			}
		}
		return nil
	}
	if err := ring.persist(manifest.ManifestVersion, candidate); err != nil {
		return err
	}
	ring.keys = candidate
	ring.manifestVersion = manifest.ManifestVersion
	return nil
}

func (ring *LocalKeyRing) activePrivate() *ecdsa.PrivateKey {
	for _, record := range ring.keys {
		if record.Descriptor.Status == KeyStatusActive {
			return ring.private[record.Descriptor.KeyID]
		}
	}
	return nil
}

func (ring *LocalKeyRing) persist(version uint64, records []localKeyRecord) error {
	raw, err := json.MarshalIndent(localKeyRingFile{
		Schema: localKeyRingSchema, ManifestVersion: version, Keys: records,
	}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicPrivateFile(ring.path, append(raw, '\n'))
}

func writeAtomicPrivateFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".publisher-key-ring-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func encodePrivateKey(private *ecdsa.PrivateKey) string {
	der, _ := x509.MarshalPKCS8PrivateKey(private)
	return base64.RawStdEncoding.EncodeToString(der)
}

func decodePrivateKey(value string) (*ecdsa.PrivateKey, error) {
	der, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || private.Curve != elliptic.P256() {
		return nil, fmt.Errorf("publisher local ring key must be ECDSA P-256")
	}
	return private, nil
}

func validateStoredKey(key VerificationKey) error {
	switch key.Status {
	case KeyStatusPreactive, KeyStatusActive, KeyStatusRetired,
		KeyStatusCompromised, KeyStatusRevoked:
	default:
		return ErrInvalidKeyTransition
	}
	candidate := key
	candidate.Status = KeyStatusActive
	if _, err := verifierFromKey(candidate); err != nil {
		return err
	}
	return nil
}

func allowedKeyTransition(from, to string) bool {
	switch from {
	case KeyStatusPreactive:
		return to == KeyStatusPreactive || to == KeyStatusActive ||
			to == KeyStatusRevoked
	case KeyStatusActive:
		return to == KeyStatusActive || to == KeyStatusRetired ||
			to == KeyStatusCompromised || to == KeyStatusRevoked
	case KeyStatusRetired, KeyStatusCompromised, KeyStatusRevoked:
		return from == to
	default:
		return false
	}
}

func samePublicKey(left, right VerificationKey) bool {
	if left.KeyID != right.KeyID || left.Algorithm != right.Algorithm ||
		left.Purpose != right.Purpose {
		return false
	}
	leftRaw, leftErr := canonicalJSON(left.PublicJWK)
	rightRaw, rightErr := canonicalJSON(right.PublicJWK)
	return leftErr == nil && rightErr == nil && string(leftRaw) == string(rightRaw)
}

func sameKeyDescriptor(left, right VerificationKey) bool {
	leftRaw, leftErr := canonicalJSON(left)
	rightRaw, rightErr := canonicalJSON(right)
	return leftErr == nil && rightErr == nil && string(leftRaw) == string(rightRaw)
}

func cloneLocalRecords(input []localKeyRecord) []localKeyRecord {
	result := make([]localKeyRecord, len(input))
	for index, record := range input {
		result[index] = localKeyRecord{
			Descriptor: cloneVerificationKey(record.Descriptor),
			PrivateKey: record.PrivateKey,
		}
	}
	return result
}

func cloneVerificationKey(key VerificationKey) VerificationKey {
	clone := key
	clone.PublicJWK = make(map[string]any, len(key.PublicJWK))
	for name, value := range key.PublicJWK {
		clone.PublicJWK[name] = value
	}
	clone.ValidUntil = cloneTime(key.ValidUntil)
	clone.RetiredAt = cloneTime(key.RetiredAt)
	clone.CompromisedAt = cloneTime(key.CompromisedAt)
	clone.RevokedAt = cloneTime(key.RevokedAt)
	return clone
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

var _ ManagedKeyProvider = (*LocalKeyRing)(nil)
