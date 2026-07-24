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
		publisherKey = flag.String("publisher-key", "", "publisher leaf PKCS8 PEM path")
		rootKey      = flag.String("root-key", "", "offline root PKCS8 PEM path")
		rootPublic   = flag.String("root-public", "", "output trust root JSON path")
		manifestPath = flag.String("manifest", "", "output signed manifest JSON path")
		version      = flag.Uint64("version", 1, "strictly increasing manifest version")
		policy       = flag.String(
			"policy-version", "publisher-attestation/v1", "verification policy version",
		)
	)
	flag.Parse()
	if strings.TrimSpace(*issuer) == "" ||
		strings.TrimSpace(*publisherKey) == "" ||
		strings.TrimSpace(*rootKey) == "" ||
		strings.TrimSpace(*rootPublic) == "" ||
		strings.TrimSpace(*manifestPath) == "" {
		flag.Usage()
		os.Exit(2)
	}

	leaf, err := publisher.LoadOrCreateLocalKey(*publisherKey)
	if err != nil {
		fatal("publisher leaf key", err)
	}
	root, err := publisher.LoadOrCreateOfflineRoot(*rootKey)
	if err != nil {
		fatal("offline root key", err)
	}
	manifest, err := publisher.SignTrustManifest(context.Background(), publisher.TrustManifest{
		Schema:          publisher.TrustManifestSchema,
		ManifestVersion: *version,
		Issuer:          strings.TrimSpace(*issuer),
		IssuedAt:        time.Now().UTC(),
		PolicyVersion:   strings.TrimSpace(*policy),
		Keys:            leaf.VerificationKeys(),
	}, root)
	if err != nil {
		fatal("sign trust manifest", err)
	}
	if err := publisher.WriteTrustBundle(
		*manifestPath, *rootPublic, manifest, root.TrustRoot(),
	); err != nil {
		fatal("write trust bundle", err)
	}
	fmt.Printf(
		"publisher trust manifest v%d written (root=%s, active=%s)\n",
		manifest.ManifestVersion, manifest.RootKeyID, manifest.Keys[0].KeyID,
	)
}

func fatal(operation string, err error) {
	fmt.Fprintf(os.Stderr, "publishertrust: %s: %v\n", operation, err)
	os.Exit(1)
}
