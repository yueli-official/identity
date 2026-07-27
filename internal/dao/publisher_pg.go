package dao

import (
	"context"
	"time"

	"github.com/yueli-official/identity/internal/publisher"
)

type publisherAttestationRow struct {
	ID                string    `orm:"id"`
	Issuer            string    `orm:"issuer"`
	PublisherSubject  string    `orm:"publisher_subject"`
	Audience          string    `orm:"audience"`
	ConsumerInstance  string    `orm:"consumer_instance"`
	Namespace         string    `orm:"namespace"`
	ArtifactKind      string    `orm:"artifact_kind"`
	ArtifactIdentity  string    `orm:"artifact_identity"`
	ArtifactVersion   string    `orm:"artifact_version"`
	ArtifactName      string    `orm:"artifact_name"`
	ArtifactURI       string    `orm:"artifact_uri"`
	ArtifactMediaType string    `orm:"artifact_media_type"`
	ArtifactSHA256    string    `orm:"artifact_sha256"`
	StatementDigest   string    `orm:"statement_digest"`
	CommandDigest     string    `orm:"command_digest"`
	KeyID             string    `orm:"key_id"`
	EnvelopeJSON      string    `orm:"envelope_json"`
	IdempotencyKey    string    `orm:"idempotency_key"`
	IssuedAt          time.Time `orm:"issued_at"`
}

func (p *PG) GetByIdempotency(
	ctx context.Context,
	issuer string,
	subject string,
	key string,
) (publisher.Attestation, bool, error) {
	var row publisherAttestationRow
	if err := p.db.Model("publisher_attestations").Ctx(ctx).
		Where("issuer", issuer).
		Where("publisher_subject", subject).
		Where("idempotency_key", key).
		Scan(&row); err != nil {
		return publisher.Attestation{}, false, err
	}
	if row.ID == "" {
		return publisher.Attestation{}, false, nil
	}
	return publisherAttestationFromRow(row), true, nil
}

func (p *PG) PutIfAbsent(
	ctx context.Context,
	value publisher.Attestation,
) (publisher.Attestation, bool, error) {
	result, err := p.db.Exec(ctx, `
INSERT INTO publisher_attestations (
    id, issuer, publisher_subject, audience, consumer_instance, namespace,
    artifact_kind, artifact_identity, artifact_version, artifact_name,
    artifact_uri, artifact_media_type, artifact_sha256, statement_digest,
    command_digest, key_id, envelope_json, idempotency_key, issued_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSONB), ?, ?)
ON CONFLICT (issuer, publisher_subject, idempotency_key) DO NOTHING
`,
		value.AttestationID, value.Issuer, value.PublisherSubject,
		value.Audience, value.ConsumerInstance, value.Namespace,
		value.Artifact.Kind, value.Artifact.Identity, value.Artifact.Version,
		value.Artifact.Name, value.Artifact.URI, value.Artifact.MediaType,
		value.Artifact.SHA256, value.StatementDigest, value.CommandDigest,
		value.KeyID, value.EnvelopeJSON, value.IdempotencyKey, value.IssuedAt,
	)
	if err != nil {
		return publisher.Attestation{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return publisher.Attestation{}, false, err
	}
	if affected == 1 {
		return value, true, nil
	}
	existing, ok, err := p.GetByIdempotency(
		ctx, value.Issuer, value.PublisherSubject, value.IdempotencyKey,
	)
	if err != nil {
		return publisher.Attestation{}, false, err
	}
	if !ok {
		return publisher.Attestation{}, false, publisher.ErrSigningUnavailable
	}
	return existing, false, nil
}

func publisherAttestationFromRow(row publisherAttestationRow) publisher.Attestation {
	return publisher.Attestation{
		AttestationID: row.ID, Issuer: row.Issuer, PublisherSubject: row.PublisherSubject,
		Audience: row.Audience, ConsumerInstance: row.ConsumerInstance, Namespace: row.Namespace,
		Artifact: publisher.Artifact{
			Kind: row.ArtifactKind, Identity: row.ArtifactIdentity, Version: row.ArtifactVersion,
			Name: row.ArtifactName, URI: row.ArtifactURI, MediaType: row.ArtifactMediaType,
			SHA256: row.ArtifactSHA256,
		},
		StatementDigest: row.StatementDigest, CommandDigest: row.CommandDigest,
		KeyID: row.KeyID, EnvelopeJSON: row.EnvelopeJSON,
		IdempotencyKey: row.IdempotencyKey, IssuedAt: row.IssuedAt,
	}
}

var _ publisher.Store = (*PG)(nil)
