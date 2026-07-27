package publisher_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

func TestKeyAdministrationRequiresPrepublishedManifestBeforeActivation(t *testing.T) {
	ctx := context.Background()
	ring, err := publisher.LoadOrCreateLocalKeyRing(t.TempDir() + "/ring.json")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := publisher.NewOfflineRoot()
	issuer := "https://identity.example.test"
	initial := signRingManifest(t, root, issuer, 1, ring.VerificationKeys())
	if err := ring.ApplyTrustManifest(ctx, initial); err != nil {
		t.Fatal(err)
	}
	state := publisher.NewTrustState(initial)
	admin, err := publisher.NewKeyAdministration(
		ring, state, []publisher.TrustRoot{root.TrustRoot()},
		t.TempDir()+"/trust-manifest.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	preactive, keys, err := admin.PrepareRotation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prepublished := signRingManifest(t, root, issuer, 2, keys)
	prepublishedRaw, _ := json.Marshal(prepublished.Manifest)
	if _, err := admin.ApplyTrustManifest(ctx, prepublishedRaw); err != nil {
		t.Fatal(err)
	}
	oldID, _ := ring.KeyID()
	if oldID == preactive.KeyID || state.Current().Manifest.ManifestVersion != 2 {
		t.Fatal("prepublishing activated the new key")
	}

	activationKeys := ring.VerificationKeys()
	now := time.Now().UTC()
	for index := range activationKeys {
		if activationKeys[index].KeyID == oldID {
			activationKeys[index].Status = publisher.KeyStatusRetired
			activationKeys[index].RetiredAt = &now
		} else {
			activationKeys[index].Status = publisher.KeyStatusActive
		}
	}
	activated := signRingManifest(t, root, issuer, 3, activationKeys)
	activatedRaw, _ := json.Marshal(activated.Manifest)
	if _, err := admin.ApplyTrustManifest(ctx, activatedRaw); err != nil {
		t.Fatal(err)
	}
	if active, _ := ring.KeyID(); active != preactive.KeyID {
		t.Fatalf("active key = %q, want %q", active, preactive.KeyID)
	}
	if state.Current().Manifest.ManifestVersion != 3 {
		t.Fatal("dynamic trust state did not advance")
	}
}

func TestLocalKeyRingPrepublishesThenActivatesAndRetainsHistory(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/publisher-key-ring.json"
	ring, err := publisher.LoadOrCreateLocalKeyRing(path)
	if err != nil {
		t.Fatal(err)
	}
	oldID, err := ring.KeyID()
	if err != nil {
		t.Fatal(err)
	}
	preactive, err := ring.PrepareRotation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if preactive.Status != publisher.KeyStatusPreactive || preactive.KeyID == oldID {
		t.Fatalf("prepared key = %#v", preactive)
	}
	if current, _ := ring.KeyID(); current != oldID {
		t.Fatalf("prepare changed active key to %q", current)
	}

	keys := ring.VerificationKeys()
	now := time.Now().UTC()
	for index := range keys {
		switch keys[index].KeyID {
		case oldID:
			keys[index].Status = publisher.KeyStatusRetired
			keys[index].RetiredAt = &now
		case preactive.KeyID:
			keys[index].Status = publisher.KeyStatusActive
		}
	}
	root, _ := publisher.NewOfflineRoot()
	signed, err := publisher.SignTrustManifest(ctx, publisher.TrustManifest{
		Schema: publisher.TrustManifestSchema, ManifestVersion: 2,
		Issuer: "https://identity.example.test", IssuedAt: now,
		PolicyVersion: "publisher-attestation/v1", Keys: keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := publisher.VerifyTrustManifest(
		ctx, signed, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ring.ApplyTrustManifest(ctx, verified); err != nil {
		t.Fatal(err)
	}
	if active, _ := ring.KeyID(); active != preactive.KeyID {
		t.Fatalf("active key = %q, want %q", active, preactive.KeyID)
	}

	reloaded, err := publisher.LoadOrCreateLocalKeyRing(path)
	if err != nil {
		t.Fatal(err)
	}
	if active, _ := reloaded.KeyID(); active != preactive.KeyID {
		t.Fatalf("reloaded active key = %q, want %q", active, preactive.KeyID)
	}
	reloadedKeys := reloaded.VerificationKeys()
	if len(reloadedKeys) != 2 ||
		reloadedKeys[0].Status != publisher.KeyStatusRetired ||
		reloadedKeys[1].Status != publisher.KeyStatusActive {
		t.Fatalf("reloaded history = %#v", reloadedKeys)
	}
}

func TestLocalKeyRingCanDisableFutureSigningButCannotReviveRetiredKey(t *testing.T) {
	ctx := context.Background()
	ring, err := publisher.LoadOrCreateLocalKeyRing(t.TempDir() + "/ring.json")
	if err != nil {
		t.Fatal(err)
	}
	keys := ring.VerificationKeys()
	now := time.Now().UTC()
	keys[0].Status = publisher.KeyStatusRetired
	keys[0].RetiredAt = &now
	root, _ := publisher.NewOfflineRoot()
	signed, err := publisher.SignTrustManifest(ctx, publisher.TrustManifest{
		Schema: publisher.TrustManifestSchema, ManifestVersion: 2,
		Issuer: "https://identity.example.test", IssuedAt: now,
		PolicyVersion: "publisher-attestation/v1", Keys: keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := publisher.VerifyTrustManifest(
		ctx, signed, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ring.ApplyTrustManifest(ctx, verified); err != nil {
		t.Fatal(err)
	}
	if _, err := ring.KeyID(); err == nil {
		t.Fatal("disabled ring still has an active signing key")
	}

	keys[0].Status = publisher.KeyStatusActive
	keys[0].RetiredAt = nil
	revive, err := publisher.SignTrustManifest(ctx, publisher.TrustManifest{
		Schema: publisher.TrustManifestSchema, ManifestVersion: 3,
		Issuer: "https://identity.example.test", IssuedAt: now.Add(time.Minute),
		PolicyVersion: "publisher-attestation/v1", Keys: keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	reviveVerified, _ := publisher.VerifyTrustManifest(
		ctx, revive, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err := ring.ApplyTrustManifest(ctx, reviveVerified); err == nil {
		t.Fatal("retired key was revived")
	}
}

func signRingManifest(
	t *testing.T,
	root *publisher.OfflineRoot,
	issuer string,
	version uint64,
	keys []publisher.VerificationKey,
) publisher.VerifiedTrustManifest {
	t.Helper()
	signed, err := publisher.SignTrustManifest(context.Background(), publisher.TrustManifest{
		Schema: publisher.TrustManifestSchema, ManifestVersion: version,
		Issuer: issuer, IssuedAt: time.Now().UTC(),
		PolicyVersion: "publisher-attestation/v1", Keys: keys,
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := publisher.VerifyTrustManifest(
		context.Background(), signed, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err != nil {
		t.Fatal(err)
	}
	return verified
}
