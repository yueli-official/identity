package githubbinding

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

func TestAuthorizeSubmissionRequiresExactAttestationAndActiveStableIDBinding(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	bindingStore := NewMemoryStore()
	bound, err := bindingStore.Bind(
		ctx, "identity-publisher",
		Account{AccountID: "123456", Login: "publisher"}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := publisher.LoadOrCreateLocalKeyRing(t.TempDir() + "/keys.json")
	if err != nil {
		t.Fatal(err)
	}
	artifact := publisher.Artifact{
		Kind: "workflow-release", Identity: "example/tool", Version: "1.2.3",
		Name: "workflow:example/tool@1.2.3", URI: "pkg:yueli-workflow/example/tool@1.2.3",
		MediaType: "application/vnd.yueli.workflow-release.v1+json",
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	issuer := "https://identity.submission.test"
	audience := "urn:yueli:registry:yotta"
	instance := "urn:yueli:platform-instance:test"
	publisherModule, err := publisher.New(publisher.Config{
		Issuer: issuer,
		Consumers: []publisher.Consumer{{
			Audience: audience, Instance: instance,
			ArtifactKinds: map[string]publisher.ArtifactPolicy{
				artifact.Kind: {MediaType: artifact.MediaType},
			},
		}},
		Store: publisher.NewMemoryStore(), Signer: keys, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := publisherModule.Issue(ctx, "identity-publisher", publisher.IssueCommand{
		IdempotencyKey: "github-submission-test-0001",
		Audience:       audience, ConsumerInstance: instance, Namespace: "example",
		Artifact: artifact,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := NewSubmissionManifest(PublisherAttestationDocument{
		AttestationID: attestation.AttestationID, Issuer: attestation.Issuer,
		PublisherSubject: attestation.PublisherSubject,
		StatementDigest:  attestation.StatementDigest, KeyID: attestation.KeyID,
		Envelope: json.RawMessage(attestation.EnvelopeJSON),
		IssuedAt: attestation.IssuedAt.Format(time.RFC3339Nano),
	}, GitHubProvenance{
		ProviderAccountID: "123456", RepositoryID: "98765",
		RepositoryNodeID: "R_98765", RepositoryFullName: "org/repository",
		PullRequestNumber: 42,
		HeadCommitSHA:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, now)
	module := newTestModule(t, bindingStore, &fakeProvider{}, now)
	policy := SubmissionPolicy{
		Issuer: issuer, Audience: audience, ConsumerInstance: instance,
		Namespace: "example", Artifact: artifact, Keys: keys.VerificationKeys(),
	}
	authorized, err := module.AuthorizeSubmission(ctx, manifest, policy)
	if err != nil {
		t.Fatal(err)
	}
	if authorized.PublisherSubject != "identity-publisher" ||
		authorized.BindingID != bound.Binding.ID || len(authorized.ManifestDigest) != 64 {
		t.Fatalf("authorization = %+v", authorized)
	}

	otherStore := NewMemoryStore()
	_, _ = otherStore.Bind(
		ctx, "other-identity", Account{AccountID: "123456", Login: "publisher"}, now,
	)
	otherModule := newTestModule(t, otherStore, &fakeProvider{}, now)
	if _, err := otherModule.AuthorizeSubmission(
		ctx, manifest, policy,
	); !errors.Is(err, ErrSubjectMismatch) {
		t.Fatalf("subject mismatch error = %v", err)
	}

	tampered := manifest
	tampered.Provenance.HeadCommitSHA = "not-a-commit"
	if _, err := module.AuthorizeSubmission(
		ctx, tampered, policy,
	); !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("invalid provenance error = %v", err)
	}
	if _, err := module.Unbind(ctx, "identity-publisher", bound.Binding.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := module.AuthorizeSubmission(
		ctx, manifest, policy,
	); !errors.Is(err, ErrBindingInactive) {
		t.Fatalf("inactive binding error = %v", err)
	}
}
