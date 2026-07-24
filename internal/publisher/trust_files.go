package publisher

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadOrCreateOfflineRoot(path string) (*OfflineRoot, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("publisher offline root key file is required")
	}
	if raw, err := os.ReadFile(path); err == nil {
		return parseOfflineRoot(raw)
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
	if err := writeExclusivePrivateFile(path, raw); errors.Is(err, os.ErrExist) {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		return parseOfflineRoot(existing)
	} else if err != nil {
		return nil, err
	}
	return newOfflineRoot(private)
}

func LoadTrustRoot(raw []byte) (TrustRoot, error) {
	var root TrustRoot
	if err := decodeStrict(raw, &root); err != nil {
		return TrustRoot{}, fmt.Errorf("%w: %v", ErrUntrustedRoot, err)
	}
	if _, err := validateTrustRoot(root); err != nil {
		return TrustRoot{}, err
	}
	return root, nil
}

func ReadTrustManifest(
	manifestPath string,
	rootPath string,
	expectedIssuer string,
	expectedActiveKeyID string,
	minimumVersion uint64,
) (VerifiedTrustManifest, error) {
	rootRaw, err := os.ReadFile(strings.TrimSpace(rootPath))
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	root, err := LoadTrustRoot(rootRaw)
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	manifestRaw, err := os.ReadFile(strings.TrimSpace(manifestPath))
	if err != nil {
		return VerifiedTrustManifest{}, err
	}
	return LoadTrustManifest(
		manifestRaw, []TrustRoot{root}, expectedIssuer, expectedActiveKeyID,
		minimumVersion,
	)
}

func WriteTrustBundle(
	manifestPath string,
	rootPath string,
	manifest TrustManifest,
	root TrustRoot,
) error {
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	rootRaw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomicPublicFile(rootPath, append(rootRaw, '\n')); err != nil {
		return err
	}
	return writeAtomicPublicFile(manifestPath, append(manifestRaw, '\n'))
}

func newOfflineRoot(private *ecdsa.PrivateKey) (*OfflineRoot, error) {
	if private == nil || private.Curve != elliptic.P256() {
		return nil, fmt.Errorf("publisher offline root must be ECDSA P-256")
	}
	keyID, err := jwkThumbprint(&private.PublicKey)
	if err != nil {
		return nil, err
	}
	return &OfflineRoot{private: private, keyID: keyID}, nil
}

func parseOfflineRoot(raw []byte) (*OfflineRoot, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("publisher offline root file has no PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("publisher offline root must be ECDSA P-256")
	}
	return newOfflineRoot(private)
}

func writeExclusivePrivateFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeAtomicPublicFile(path string, raw []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("publisher trust output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".publisher-trust-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o644); err != nil {
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
