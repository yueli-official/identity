package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

func TestReusableTrustManifest(t *testing.T) {
	directory := t.TempDir()
	leaves, err := publisher.LoadOrCreateLocalKeyRing(filepath.Join(directory, "leaf.json"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := publisher.LoadOrCreateOfflineRoot(filepath.Join(directory, "root.pem"))
	if err != nil {
		t.Fatal(err)
	}
	keys := leaves.VerificationKeys()
	manifest, err := publisher.SignTrustManifest(context.Background(), publisher.TrustManifest{
		Schema: publisher.TrustManifestSchema, ManifestVersion: 3,
		Issuer: "https://identity.example.com", IssuedAt: time.Unix(1_700_000_000, 0).UTC(),
		PolicyVersion: "publisher-attestation/v1", Keys: keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	rootPath := filepath.Join(directory, "root.json")
	if err := publisher.WriteTrustBundle(manifestPath, rootPath, manifest, root.TrustRoot()); err != nil {
		t.Fatal(err)
	}
	first, ok := reusableTrustManifest(
		manifestPath, rootPath, manifest.Issuer, manifest.PolicyVersion,
		manifest.ManifestVersion, keys, root.TrustRoot(),
	)
	if !ok || first.SnapshotHash == "" {
		t.Fatalf("expected reusable manifest, got ok=%t snapshot=%q", ok, first.SnapshotHash)
	}
	if _, ok := reusableTrustManifest(
		manifestPath, rootPath, manifest.Issuer, "publisher-attestation/v2",
		manifest.ManifestVersion, keys, root.TrustRoot(),
	); ok {
		t.Fatal("changed policy must require a new manifest")
	}
}
