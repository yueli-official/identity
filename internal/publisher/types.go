// Package publisher issues and verifies long-lived publisher attestations.
// It deliberately owns a signing domain separate from OIDC and access tokens.
package publisher

import (
	"context"
	"errors"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const (
	StatementType = "https://in-toto.io/Statement/v1"
	PredicateType = "https://yueli.dev/attestations/publisher-intent/v1"
	PayloadType   = "application/vnd.in-toto+json"
	KeyPurpose    = "publisher-attestation"

	KeyStatusPreactive   = "preactive"
	KeyStatusActive      = "active"
	KeyStatusRetired     = "retired"
	KeyStatusCompromised = "compromised"
	KeyStatusRevoked     = "revoked"
)

var (
	ErrInvalidCommand       = errors.New("publisher attestation command is invalid")
	ErrInvalidAttestation   = errors.New("publisher attestation is invalid")
	ErrConsumerNotFound     = errors.New("publisher consumer is not registered")
	ErrConsumerDisabled     = errors.New("publisher consumer is disabled")
	ErrIdempotencyConflict  = errors.New("publisher idempotency key was reused with different content")
	ErrSigningUnavailable   = errors.New("publisher signing is unavailable")
	ErrInvalidTrustManifest = errors.New("publisher trust manifest is invalid")
	ErrUntrustedRoot        = errors.New("publisher trust root is not trusted")
)

type ArtifactPolicy struct {
	MediaType string
}

type Consumer struct {
	Audience      string
	Instance      string
	Disabled      bool
	ArtifactKinds map[string]ArtifactPolicy
}

type Artifact struct {
	Kind      string `json:"kind"`
	Identity  string `json:"identity"`
	Version   string `json:"version"`
	Name      string `json:"name"`
	URI       string `json:"uri"`
	MediaType string `json:"mediaType"`
	SHA256    string `json:"sha256"`
}

type IssueCommand struct {
	IdempotencyKey   string   `json:"idempotencyKey"`
	Audience         string   `json:"audience"`
	ConsumerInstance string   `json:"consumerInstance"`
	Namespace        string   `json:"namespace"`
	Artifact         Artifact `json:"artifact"`
}

type Attestation struct {
	AttestationID    string    `json:"attestationId"`
	Issuer           string    `json:"issuer"`
	PublisherSubject string    `json:"publisherSubject"`
	StatementDigest  string    `json:"statementDigest"`
	CommandDigest    string    `json:"-"`
	KeyID            string    `json:"keyId"`
	EnvelopeJSON     string    `json:"envelope"`
	IssuedAt         time.Time `json:"issuedAt"`
	IdempotencyKey   string    `json:"-"`
	Audience         string    `json:"-"`
	ConsumerInstance string    `json:"-"`
	Namespace        string    `json:"-"`
	Artifact         Artifact  `json:"-"`
}

type VerificationKey struct {
	KeyID            string         `json:"keyId"`
	Algorithm        string         `json:"algorithm"`
	Purpose          string         `json:"purpose"`
	Status           string         `json:"status"`
	PublicJWK        map[string]any `json:"publicJwk"`
	ActivatedAt      time.Time      `json:"validFrom"`
	ValidUntil       *time.Time     `json:"validUntil,omitempty"`
	RetiredAt        *time.Time     `json:"retiredAt,omitempty"`
	CompromisedAt    *time.Time     `json:"compromisedAt,omitempty"`
	RevokedAt        *time.Time     `json:"revokedAt,omitempty"`
	RevocationReason string         `json:"revocationReason,omitempty"`
}

type VerificationPolicy struct {
	Issuer           string
	Audience         string
	ConsumerInstance string
	Namespace        string
	Artifact         Artifact
	Keys             []VerificationKey
}

type VerifiedStatement struct {
	AttestationID    string
	PublisherSubject string
	IssuedAt         time.Time
	StatementDigest  string
}

type Store interface {
	GetByIdempotency(context.Context, string, string, string) (Attestation, bool, error)
	PutIfAbsent(context.Context, Attestation) (Attestation, bool, error)
}

type KeyProvider interface {
	dsse.Signer
	VerificationKeys() []VerificationKey
}

type Config struct {
	Issuer    string
	Consumers []Consumer
	Store     Store
	Signer    KeyProvider
	Now       func() time.Time
}

type statement struct {
	Type          string    `json:"_type"`
	Subject       []subject `json:"subject"`
	PredicateType string    `json:"predicateType"`
	Predicate     predicate `json:"predicate"`
}

type subject struct {
	Name      string            `json:"name"`
	URI       string            `json:"uri"`
	MediaType string            `json:"mediaType"`
	Digest    map[string]string `json:"digest"`
}

type predicate struct {
	AttestationID    string            `json:"attestationId"`
	Issuer           string            `json:"issuer"`
	Purpose          string            `json:"purpose"`
	Publisher        publisherIdentity `json:"publisher"`
	Audience         string            `json:"audience"`
	ConsumerInstance string            `json:"consumerInstance"`
	Namespace        string            `json:"namespace"`
	Artifact         artifactIdentity  `json:"artifact"`
	IssuedAt         string            `json:"issuedAt"`
}

type publisherIdentity struct {
	Subject string `json:"subject"`
}

type artifactIdentity struct {
	Kind     string `json:"kind"`
	Identity string `json:"identity"`
	Version  string `json:"version"`
}
