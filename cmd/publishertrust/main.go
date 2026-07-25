// Command publishertrust creates a root-signed Publisher Attestation trust
// snapshot. It is an offline operator tool; Identity runtime never loads the
// root private key.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"platform/services/identity/internal/publisher"
)

func main() {
	var (
		issuer       = flag.String("issuer", "", "Identity issuer URI")
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
	fmt.Printf(
		"publisher trust manifest v%d written (root=%s, snapshot=%s)\nstep-up resource: publisher:trust-manifest:%s\n",
		manifest.ManifestVersion, manifest.RootKeyID, verified.SnapshotHash,
		verified.SnapshotHash,
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
