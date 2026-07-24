package publisher_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platform/services/identity/internal/publisher"
)

func TestIssueAndVerifyPublisherAttestation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 1, 2, 3, 0, time.UTC)
	keys, err := publisher.NewLocalKeyProvider()
	if err != nil {
		t.Fatal(err)
	}
	module, err := publisher.New(publisher.Config{
		Issuer: "https://identity.example.test",
		Consumers: []publisher.Consumer{{
			Audience: "urn:yueli:registry:yotta",
			Instance: "urn:yueli:platform-instance:test",
			ArtifactKinds: map[string]publisher.ArtifactPolicy{
				"workflow-release": {
					MediaType: "application/vnd.yueli.workflow-release.v1+json",
				},
			},
		}},
		Store:  publisher.NewMemoryStore(),
		Signer: keys,
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	command := publisher.IssueCommand{
		IdempotencyKey:   "publisher-test-request-0001",
		Audience:         "urn:yueli:registry:yotta",
		ConsumerInstance: "urn:yueli:platform-instance:test",
		Namespace:        "example",
		Artifact: publisher.Artifact{
			Kind:      "workflow-release",
			Identity:  "example/resize-images",
			Version:   "1.4.0",
			Name:      "workflow:example/resize-images@1.4.0",
			URI:       "pkg:yueli-workflow/example/resize-images@1.4.0",
			MediaType: "application/vnd.yueli.workflow-release.v1+json",
			SHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}

	first, err := module.Issue(context.Background(), "identity-user-1", command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := module.Issue(context.Background(), "identity-user-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if first.EnvelopeJSON != second.EnvelopeJSON || first.StatementDigest != second.StatementDigest {
		t.Fatal("idempotent issue returned a different attestation")
	}

	verified, err := publisher.Verify(context.Background(), first, publisher.VerificationPolicy{
		Issuer:           "https://identity.example.test",
		Audience:         command.Audience,
		ConsumerInstance: command.ConsumerInstance,
		Namespace:        command.Namespace,
		Artifact:         command.Artifact,
		Keys:             keys.VerificationKeys(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if verified.PublisherSubject != "identity-user-1" {
		t.Fatalf("publisher subject = %q", verified.PublisherSubject)
	}
	if verified.AttestationID == "" || !verified.IssuedAt.Equal(now) {
		t.Fatalf("verified statement = %#v", verified)
	}
	metadataTampered := first
	metadataTampered.PublisherSubject = "identity-user-2"
	if _, err := publisher.Verify(context.Background(), metadataTampered, publisher.VerificationPolicy{
		Issuer: "https://identity.example.test", Audience: command.Audience,
		ConsumerInstance: command.ConsumerInstance, Namespace: command.Namespace,
		Artifact: command.Artifact, Keys: keys.VerificationKeys(),
	}); !errors.Is(err, publisher.ErrInvalidAttestation) {
		t.Fatalf("metadata-tampered Verify() error = %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal([]byte(first.EnvelopeJSON), &envelope); err != nil {
		t.Fatal(err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope["payload"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var statement map[string]any
	if err := json.Unmarshal(payload, &statement); err != nil {
		t.Fatal(err)
	}
	statement["predicate"].(map[string]any)["audience"] = "urn:yueli:registry:other"
	tampered, err := json.Marshal(statement)
	if err != nil {
		t.Fatal(err)
	}
	envelope["payload"] = base64.StdEncoding.EncodeToString(tampered)
	tamperedEnvelope, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	first.EnvelopeJSON = string(tamperedEnvelope)
	if _, err := publisher.Verify(context.Background(), first, publisher.VerificationPolicy{
		Issuer: first.Issuer, Audience: command.Audience, ConsumerInstance: command.ConsumerInstance,
		Namespace: command.Namespace, Artifact: command.Artifact, Keys: keys.VerificationKeys(),
	}); !errors.Is(err, publisher.ErrInvalidAttestation) {
		t.Fatalf("tampered Verify() error = %v", err)
	}
}

func TestIssueRejectsIdempotencyKeyReuseWithDifferentArtifact(t *testing.T) {
	t.Parallel()

	keys, err := publisher.NewLocalKeyProvider()
	if err != nil {
		t.Fatal(err)
	}
	module, err := publisher.New(publisher.Config{
		Issuer: "https://identity.example.test",
		Consumers: []publisher.Consumer{{
			Audience: "urn:yueli:registry:yotta", Instance: "urn:yueli:platform-instance:test",
			ArtifactKinds: map[string]publisher.ArtifactPolicy{
				"workflow-release": {MediaType: "application/vnd.yueli.workflow-release.v1+json"},
			},
		}},
		Store: publisher.NewMemoryStore(), Signer: keys,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := publisher.IssueCommand{
		IdempotencyKey: "publisher-test-request-0002",
		Audience:       "urn:yueli:registry:yotta", ConsumerInstance: "urn:yueli:platform-instance:test",
		Namespace: "example",
		Artifact: publisher.Artifact{
			Kind: "workflow-release", Identity: "example/tool", Version: "1.0.0",
			Name: "workflow:example/tool@1.0.0", URI: "pkg:yueli-workflow/example/tool@1.0.0",
			MediaType: "application/vnd.yueli.workflow-release.v1+json",
			SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	if _, err := module.Issue(context.Background(), "identity-user-1", command); err != nil {
		t.Fatal(err)
	}
	command.Artifact.SHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := module.Issue(context.Background(), "identity-user-1", command); !errors.Is(err, publisher.ErrIdempotencyConflict) {
		t.Fatalf("Issue() error = %v", err)
	}
}

func TestLocalKeyFileKeepsStablePublicIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "publisher-signing-key.pem")
	first, err := publisher.LoadOrCreateLocalKey(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := publisher.LoadOrCreateLocalKey(path)
	if err != nil {
		t.Fatal(err)
	}
	firstID, _ := first.KeyID()
	secondID, _ := second.KeyID()
	if firstID == "" || firstID != secondID {
		t.Fatalf("key IDs are not stable: %q != %q", firstID, secondID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("BEGIN PRIVATE KEY")) {
		t.Fatal("local key file is not PKCS8 PEM")
	}
	if keys := first.VerificationKeys(); len(keys) != 1 || keys[0].PublicJWK["d"] != nil {
		t.Fatalf("verification keys exposed private material: %#v", keys)
	}
}
