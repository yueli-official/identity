package testidentity

import (
	"context"
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	foundationauth "github.com/yueli-official/foundation/go/auth"
)

const (
	testIssuer = "https://identity.test"
	testKeyID  = "test-identity-key"
)

type keySource struct {
	public ed25519.PublicKey
}

func (source keySource) PublicKey(context.Context, string) (any, error) {
	return source.public, nil
}

func Principal(t testing.TB, kind, subject, clientID string, roles, scopes []string) *foundationauth.Principal {
	t.Helper()
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: private},
		(&jose.SignerOptions{}).WithHeader("kid", testKeyID),
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	raw, err := jwt.Signed(signer).
		Claims(jwt.Claims{
			Issuer: testIssuer, Subject: subject,
			IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(time.Minute)),
		}).
		Claims(map[string]any{
			"subject_kind": kind, "client_id": clientID,
			"roles": roles, "scope": strings.Join(scopes, " "),
		}).
		Serialize()
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := foundationauth.NewVerifier(foundationauth.Config{
		Keys: keySource{public: private.Public().(ed25519.PublicKey)}, Issuer: testIssuer,
		Algorithms: []jose.SignatureAlgorithm{jose.EdDSA},
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	return principal
}

func User(t testing.TB, subject string, roles, scopes []string) *foundationauth.Principal {
	t.Helper()
	return Principal(t, "user", subject, "", roles, scopes)
}

func Client(t testing.TB, clientID string, scopes []string) *foundationauth.Principal {
	t.Helper()
	return Principal(t, "client", "", clientID, nil, scopes)
}
