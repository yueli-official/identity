package githubbinding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

const SubmissionManifestSchema = "https://yueli.dev/registry/github-submission/v1"

var (
	decimalIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,39}$`)
	commitSHAPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

type PublisherAttestationDocument struct {
	AttestationID    string          `json:"attestationId"`
	Issuer           string          `json:"issuer"`
	PublisherSubject string          `json:"publisherSubject"`
	StatementDigest  string          `json:"statementDigest"`
	KeyID            string          `json:"keyId"`
	Envelope         json.RawMessage `json:"envelope"`
	IssuedAt         string          `json:"issuedAt"`
}

type GitHubProvenance struct {
	Provider           string `json:"provider"`
	ProviderAccountID  string `json:"providerAccountId"`
	RepositoryID       string `json:"repositoryId"`
	RepositoryNodeID   string `json:"repositoryNodeId,omitempty"`
	RepositoryFullName string `json:"repositoryFullName"`
	PullRequestNumber  int64  `json:"pullRequestNumber"`
	HeadCommitSHA      string `json:"headCommitSha"`
}

type SubmissionManifest struct {
	Schema               string                       `json:"schema"`
	PublisherAttestation PublisherAttestationDocument `json:"publisherAttestation"`
	Provenance           GitHubProvenance             `json:"provenance"`
	CreatedAt            string                       `json:"createdAt"`
}

type SubmissionPolicy struct {
	Issuer           string
	Audience         string
	ConsumerInstance string
	Namespace        string
	Artifact         publisher.Artifact
	Keys             []publisher.VerificationKey
}

type AuthorizedSubmission struct {
	Manifest          SubmissionManifest
	ManifestDigest    string
	PublisherSubject  string
	BindingID         string
	BindingVerifiedAt time.Time
}

// NewSubmissionManifest constructs the portable registration document. The
// repository/PR/commit values must come from the consumer's verified GitHub App
// event/API response; this function never interprets browser-supplied ownership.
func NewSubmissionManifest(
	attestation PublisherAttestationDocument,
	provenance GitHubProvenance,
	createdAt time.Time,
) SubmissionManifest {
	provenance.Provider = ProviderGitHub
	return SubmissionManifest{
		Schema: SubmissionManifestSchema, PublisherAttestation: attestation,
		Provenance: provenance, CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
	}
}

// AuthorizeSubmission verifies the exact Publisher Attestation target and then
// requires its publisher subject to equal the active stable-ID GitHub binding.
func (module *Module) AuthorizeSubmission(
	ctx context.Context,
	manifest SubmissionManifest,
	policy SubmissionPolicy,
) (AuthorizedSubmission, error) {
	if err := validateSubmissionManifest(manifest); err != nil {
		return AuthorizedSubmission{}, err
	}
	binding, err := module.store.FindActiveByAccount(
		ctx, manifest.Provenance.ProviderAccountID,
	)
	if err != nil {
		return AuthorizedSubmission{}, err
	}
	issuedAt, err := time.Parse(
		time.RFC3339Nano, manifest.PublisherAttestation.IssuedAt,
	)
	if err != nil {
		return AuthorizedSubmission{}, ErrInvalidSubmission
	}
	keys := make([]publisher.VerificationKey, 0, len(policy.Keys))
	for _, key := range policy.Keys {
		if key.Status == publisher.KeyStatusActive ||
			key.Status == publisher.KeyStatusRetired {
			keys = append(keys, key)
		}
	}
	document := manifest.PublisherAttestation
	verified, err := publisher.Verify(ctx, publisher.Attestation{
		AttestationID: document.AttestationID, Issuer: document.Issuer,
		PublisherSubject: document.PublisherSubject,
		StatementDigest:  document.StatementDigest, KeyID: document.KeyID,
		EnvelopeJSON: string(document.Envelope), IssuedAt: issuedAt,
	}, publisher.VerificationPolicy{
		Issuer: policy.Issuer, Audience: policy.Audience,
		ConsumerInstance: policy.ConsumerInstance, Namespace: policy.Namespace,
		Artifact: policy.Artifact, Keys: keys,
	})
	if err != nil {
		return AuthorizedSubmission{}, ErrInvalidSubmission
	}
	publisherSubject, err := module.resolvePublisherSubject(ctx, binding.IdentityID)
	if err != nil {
		return AuthorizedSubmission{}, ErrBindingInactive
	}
	if verified.PublisherSubject != publisherSubject {
		return AuthorizedSubmission{}, ErrSubjectMismatch
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return AuthorizedSubmission{}, ErrInvalidSubmission
	}
	digest := sha256.Sum256(encoded)
	return AuthorizedSubmission{
		Manifest: manifest, ManifestDigest: hex.EncodeToString(digest[:]),
		PublisherSubject: verified.PublisherSubject, BindingID: binding.ID,
		BindingVerifiedAt: binding.LastVerifiedAt,
	}, nil
}

func validateSubmissionManifest(manifest SubmissionManifest) error {
	provenance := manifest.Provenance
	if manifest.Schema != SubmissionManifestSchema ||
		provenance.Provider != ProviderGitHub ||
		!decimalIDPattern.MatchString(provenance.ProviderAccountID) ||
		!decimalIDPattern.MatchString(provenance.RepositoryID) ||
		strings.TrimSpace(provenance.RepositoryFullName) == "" ||
		provenance.PullRequestNumber <= 0 ||
		!commitSHAPattern.MatchString(provenance.HeadCommitSHA) {
		return ErrInvalidSubmission
	}
	if _, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt); err != nil {
		return ErrInvalidSubmission
	}
	return nil
}
