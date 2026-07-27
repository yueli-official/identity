package publisher_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

func TestOfflineRootSignedTrustManifestRoundTrip(t *testing.T) {
	now := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	root, err := publisher.NewOfflineRoot()
	if err != nil {
		t.Fatal(err)
	}
	active, err := publisher.NewLocalKeyProvider()
	if err != nil {
		t.Fatal(err)
	}
	retired, err := publisher.NewLocalKeyProvider()
	if err != nil {
		t.Fatal(err)
	}
	keys := append(active.VerificationKeys(), retired.VerificationKeys()...)
	keys[1].Status = publisher.KeyStatusRetired
	retiredAt := now.Add(-time.Hour)
	keys[1].RetiredAt = &retiredAt

	signed, err := publisher.SignTrustManifest(context.Background(), publisher.TrustManifest{
		Schema:          publisher.TrustManifestSchema,
		ManifestVersion: 7,
		Issuer:          "https://identity.example.test",
		IssuedAt:        now,
		PolicyVersion:   "publisher-attestation/v1",
		Keys:            keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if signed.RootKeyID == "" || signed.Signature == "" {
		t.Fatalf("signed manifest lacks root proof: %#v", signed)
	}

	verified, err := publisher.VerifyTrustManifest(
		context.Background(), signed, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if verified.SnapshotHash == "" || verified.Manifest.ManifestVersion != 7 {
		t.Fatalf("verified manifest = %#v", verified)
	}

	raw, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := publisher.LoadTrustManifest(
		raw,
		[]publisher.TrustRoot{root.TrustRoot()},
		"https://identity.example.test",
		active.VerificationKeys()[0].KeyID,
		7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SnapshotHash != verified.SnapshotHash {
		t.Fatalf("loaded snapshot hash %q != %q", loaded.SnapshotHash, verified.SnapshotHash)
	}
	if _, err := publisher.LoadTrustManifest(
		raw, []publisher.TrustRoot{root.TrustRoot()},
		"https://identity.example.test", active.VerificationKeys()[0].KeyID, 8,
	); !errors.Is(err, publisher.ErrInvalidTrustManifest) {
		t.Fatalf("rollback LoadTrustManifest() error = %v", err)
	}

	directory := t.TempDir()
	manifestPath := filepath.Join(directory, "trust-manifest.json")
	rootPath := filepath.Join(directory, "trust-root.json")
	if err := publisher.WriteTrustBundle(
		manifestPath, rootPath, signed, root.TrustRoot(),
	); err != nil {
		t.Fatal(err)
	}
	fromFiles, err := publisher.ReadTrustManifest(
		manifestPath, rootPath, "https://identity.example.test",
		active.VerificationKeys()[0].KeyID, 7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fromFiles.SnapshotHash != verified.SnapshotHash {
		t.Fatalf("file snapshot hash %q != %q", fromFiles.SnapshotHash, verified.SnapshotHash)
	}
}

func TestTrustManifestRejectsTamperUnknownRootAndMultipleActiveKeys(t *testing.T) {
	root, _ := publisher.NewOfflineRoot()
	otherRoot, _ := publisher.NewOfflineRoot()
	first, _ := publisher.NewLocalKeyProvider()
	second, _ := publisher.NewLocalKeyProvider()
	manifest := publisher.TrustManifest{
		Schema:          publisher.TrustManifestSchema,
		ManifestVersion: 1,
		Issuer:          "https://identity.example.test",
		IssuedAt:        time.Now().UTC(),
		PolicyVersion:   "publisher-attestation/v1",
		Keys:            append(first.VerificationKeys(), second.VerificationKeys()...),
	}
	if _, err := publisher.SignTrustManifest(context.Background(), manifest, root); !errors.Is(
		err, publisher.ErrInvalidTrustManifest,
	) {
		t.Fatalf("multiple-active SignTrustManifest() error = %v", err)
	}

	manifest.Keys = first.VerificationKeys()
	signed, err := publisher.SignTrustManifest(context.Background(), manifest, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.VerifyTrustManifest(
		context.Background(), signed, []publisher.TrustRoot{otherRoot.TrustRoot()},
	); !errors.Is(err, publisher.ErrUntrustedRoot) {
		t.Fatalf("unknown-root VerifyTrustManifest() error = %v", err)
	}

	signed.Keys[0].Status = publisher.KeyStatusRevoked
	if _, err := publisher.VerifyTrustManifest(
		context.Background(), signed, []publisher.TrustRoot{root.TrustRoot()},
	); !errors.Is(err, publisher.ErrInvalidTrustManifest) {
		t.Fatalf("tampered VerifyTrustManifest() error = %v", err)
	}
}

func TestOfflineRootFileKeepsStableTrustAnchor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "offline-root.pem")
	first, err := publisher.LoadOrCreateOfflineRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := publisher.LoadOrCreateOfflineRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.TrustRoot().KeyID == "" ||
		first.TrustRoot().KeyID != second.TrustRoot().KeyID {
		t.Fatalf(
			"offline trust root changed: %q != %q",
			first.TrustRoot().KeyID, second.TrustRoot().KeyID,
		)
	}
}
