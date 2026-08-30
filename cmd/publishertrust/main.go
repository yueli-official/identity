// Command publishertrust creates a root-signed Publisher Attestation trust
// snapshot. It is an offline operator tool; Identity runtime never loads the
// root private key.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

func main() {
	var (
		issuer       = flag.String("issuer", os.Getenv("IDENTITY_ISSUER"), "Identity issuer URI (or IDENTITY_ISSUER)")
		keyRing      = flag.String("key-ring", "", "publisher local key ring JSON path")
		rootKey      = flag.String("root-key", "", "offline root PKCS8 PEM path")
		rootPublic   = flag.String("root-public", "", "output trust root JSON path")
		manifestPath = flag.String("manifest", "", "output signed manifest JSON path")
		version      = flag.Uint64("version", 1, "strictly increasing manifest version")
		activateKey  = flag.String("activate-key", "", "activate a preactive key and retire the current active key")
		disable      = flag.Bool("disable-signing", false, "retire the active key without activating a replacement")
		policy       = flag.String(
			"policy-version", "publisher-attestation/v1", "verification policy version",
		)
	)
	flag.Parse()
	if strings.TrimSpace(*issuer) == "" ||
		strings.TrimSpace(*keyRing) == "" ||
		strings.TrimSpace(*rootKey) == "" ||
		strings.TrimSpace(*rootPublic) == "" ||
		strings.TrimSpace(*manifestPath) == "" {
		flag.Usage()
		os.Exit(2)
	}
	if strings.TrimSpace(*activateKey) != "" && *disable {
		fatal("flags", fmt.Errorf("-activate-key and -disable-signing are mutually exclusive"))
	}

	leaf, err := publisher.LoadOrCreateLocalKeyRing(*keyRing)
	if err != nil {
		fatal("publisher leaf key", err)
	}
	root, err := publisher.LoadOrCreateOfflineRoot(*rootKey)
	if err != nil {
		fatal("offline root key", err)
	}
	keys := leaf.VerificationKeys()
	if err := applyRequestedTransition(keys, strings.TrimSpace(*activateKey), *disable); err != nil {
		fatal("key transition", err)
	}
	if strings.TrimSpace(*activateKey) == "" && !*disable {
		if existing, ok := reusableTrustManifest(
			*manifestPath, *rootPublic, strings.TrimSpace(*issuer),
			strings.TrimSpace(*policy), *version, keys, root.TrustRoot(),
		); ok {
			printTrustManifest("already current", existing)
			return
		}
	}
	manifest, err := publisher.SignTrustManifest(context.Background(), publisher.TrustManifest{
		Schema:          publisher.TrustManifestSchema,
		ManifestVersion: *version,
		Issuer:          strings.TrimSpace(*issuer),
		IssuedAt:        time.Now().UTC(),
		PolicyVersion:   strings.TrimSpace(*policy),
		Keys:            keys,
	}, root)
	if err != nil {
		fatal("sign trust manifest", err)
	}
	verified, err := publisher.VerifyTrustManifest(
		context.Background(), manifest, []publisher.TrustRoot{root.TrustRoot()},
	)
	if err != nil {
		fatal("verify trust manifest", err)
	}
	if err := publisher.WriteTrustBundle(
		*manifestPath, *rootPublic, manifest, root.TrustRoot(),
	); err != nil {
		fatal("write trust bundle", err)
	}
	printTrustManifest("written", verified)
}

func reusableTrustManifest(
	manifestPath, rootPath, issuer, policy string,
	version uint64,
	keys []publisher.VerificationKey,
	root publisher.TrustRoot,
) (publisher.VerifiedTrustManifest, bool) {
	activeKeyID := ""
	for _, key := range keys {
		if key.Status == publisher.KeyStatusActive {
			activeKeyID = key.KeyID
			break
		}
	}
	verified, err := publisher.ReadTrustManifest(
		manifestPath, rootPath, issuer, activeKeyID, version,
	)
	if err != nil {
		return publisher.VerifiedTrustManifest{}, false
	}
	manifest := verified.Manifest
	if manifest.ManifestVersion != version ||
		manifest.Issuer != issuer ||
		manifest.PolicyVersion != policy ||
		manifest.RootKeyID != root.KeyID ||
		!reflect.DeepEqual(manifest.Keys, keys) {
		return publisher.VerifiedTrustManifest{}, false
	}
	return verified, true
}

func printTrustManifest(action string, verified publisher.VerifiedTrustManifest) {
	manifest := verified.Manifest
	fmt.Printf(
		"publisher trust manifest v%d %s (root=%s, snapshot=%s)\nstep-up resource: publisher:trust-manifest:%s\n",
		manifest.ManifestVersion, action, manifest.RootKeyID,
		verified.SnapshotHash, verified.SnapshotHash,
	)
}

func applyRequestedTransition(keys []publisher.VerificationKey, activate string, disable bool) error {
	if activate == "" && !disable {
		return nil
	}
	now := time.Now().UTC()
	found := false
	for index := range keys {
		switch {
		case activate != "" && keys[index].KeyID == activate:
			if keys[index].Status != publisher.KeyStatusPreactive {
				return fmt.Errorf("key %s is not preactive", activate)
			}
			keys[index].Status = publisher.KeyStatusActive
			found = true
		case keys[index].Status == publisher.KeyStatusActive:
			keys[index].Status = publisher.KeyStatusRetired
			keys[index].RetiredAt = &now
		}
	}
	if activate != "" && !found {
		return fmt.Errorf("preactive key %s not found", activate)
	}
	return nil
}

func fatal(operation string, err error) {
	fmt.Fprintf(os.Stderr, "publishertrust: %s: %v\n", operation, err)
	os.Exit(1)
}
