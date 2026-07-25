package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type GitHubSubmissionAttestation struct {
	AttestationID    string          `json:"attestationId" v:"required|uuid"`
	Issuer           string          `json:"issuer" v:"required|max-length:512"`
	PublisherSubject string          `json:"publisherSubject" v:"required|max-length:128"`
	StatementDigest  string          `json:"statementDigest" v:"required|length:64,64"`
	KeyID            string          `json:"keyId" v:"required|max-length:256"`
	Envelope         json.RawMessage `json:"envelope" v:"required"`
	IssuedAt         string          `json:"issuedAt" v:"required|max-length:64"`
}

type GitHubSubmissionProvenance struct {
	ProviderAccountID  string `json:"providerAccountId" v:"required|max-length:40"`
	RepositoryID       string `json:"repositoryId" v:"required|max-length:40"`
	RepositoryNodeID   string `json:"repositoryNodeId" v:"max-length:128"`
	RepositoryFullName string `json:"repositoryFullName" v:"required|max-length:256"`
	PullRequestNumber  int64  `json:"pullRequestNumber" v:"required|min:1"`
	HeadCommitSHA      string `json:"headCommitSha" v:"required|max-length:64"`
}

type AuthorizeGitHubSubmissionReq struct {
	g.Meta           `path:"/api/internal/publisher/github-submissions" method:"post" tags:"publisher,github,internal" summary:"Authorize and build a GitHub PR submission manifest" security:"MachineAuth"`
	Audience         string                      `json:"audience" v:"required|max-length:512"`
	ConsumerInstance string                      `json:"consumerInstance" v:"required|max-length:512"`
	Namespace        string                      `json:"namespace" v:"required|max-length:256"`
	Artifact         PublisherArtifact           `json:"artifact" v:"required"`
	Attestation      GitHubSubmissionAttestation `json:"attestation" v:"required"`
	Provenance       GitHubSubmissionProvenance  `json:"provenance" v:"required"`
}

type AuthorizeGitHubSubmissionRes struct {
	Authorized        bool            `json:"authorized"`
	PublisherSubject  string          `json:"publisherSubject"`
	BindingID         string          `json:"bindingId"`
	BindingVerifiedAt string          `json:"bindingVerifiedAt"`
	ManifestDigest    string          `json:"manifestDigest"`
	Manifest          json.RawMessage `json:"manifest"`
}
