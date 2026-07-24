package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type PublisherArtifact struct {
	Kind      string `json:"kind" v:"required|max-length:256"`
	Identity  string `json:"identity" v:"required|max-length:256"`
	Version   string `json:"version" v:"required|max-length:256"`
	Name      string `json:"name" v:"required|max-length:512"`
	URI       string `json:"uri" v:"required|max-length:1024"`
	MediaType string `json:"mediaType" v:"required|max-length:256"`
	SHA256    string `json:"sha256" v:"required|length:64,64"`
}

type IssuePublisherAttestationReq struct {
	g.Meta           `path:"/api/v1/account/publisher-attestations" method:"post" tags:"publisher" summary:"Issue a publisher attestation" security:"UserAuth"`
	IdempotencyKey   string            `json:"idempotencyKey" v:"required|min-length:16|max-length:200"`
	Audience         string            `json:"audience" v:"required|max-length:512"`
	ConsumerInstance string            `json:"consumerInstance" v:"required|max-length:512"`
	Namespace        string            `json:"namespace" v:"required|max-length:256"`
	Artifact         PublisherArtifact `json:"artifact" v:"required"`
}

type IssuePublisherAttestationRes struct {
	AttestationID    string          `json:"attestationId"`
	Issuer           string          `json:"issuer"`
	PublisherSubject string          `json:"publisherSubject"`
	StatementDigest  string          `json:"statementDigest"`
	KeyID            string          `json:"keyId"`
	Envelope         json.RawMessage `json:"envelope"`
	IssuedAt         string          `json:"issuedAt"`
}

type PublisherVerificationKeysReq struct {
	g.Meta `path:"/api/v1/publisher/verification-keys" method:"get" tags:"publisher" summary:"Get publisher attestation verification keys"`
}

type PublisherVerificationKey struct {
	KeyID       string         `json:"keyId"`
	Algorithm   string         `json:"algorithm"`
	Purpose     string         `json:"purpose"`
	Status      string         `json:"status"`
	PublicJWK   map[string]any `json:"publicJwk"`
	ActivatedAt string         `json:"activatedAt"`
	RetiredAt   string         `json:"retiredAt,omitempty"`
}

type PublisherVerificationKeysRes struct {
	ManifestVersion int                        `json:"manifestVersion"`
	Keys            []PublisherVerificationKey `json:"keys"`
}
