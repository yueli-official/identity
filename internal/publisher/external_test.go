package publisher_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"platform/services/identity/internal/publisher"
)

func TestSecretAndCryptoSignerProvidersExposeOnlyPublicMaterial(t *testing.T) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	secret, err := publisher.NewSecretPEMKeyProvider(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := publisher.NewCryptoSignerProvider(private, secret.VerificationKeys()[0].ActivatedAt)
	if err != nil {
		t.Fatal(err)
	}
	secretID, _ := secret.KeyID()
	remoteID, _ := remote.KeyID()
	if secretID == "" || secretID != remoteID {
		t.Fatalf("provider key IDs differ: %q != %q", secretID, remoteID)
	}
	if secret.VerificationKeys()[0].PublicJWK["d"] != nil {
		t.Fatal("secret provider exposed private JWK material")
	}
	if _, err := remote.Sign(context.Background(), []byte("publisher payload")); err != nil {
		t.Fatal(err)
	}
}
