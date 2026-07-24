package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

func Verify(
	ctx context.Context,
	attestation Attestation,
	policy VerificationPolicy,
) (VerifiedStatement, error) {
	var envelope dsse.Envelope
	if err := decodeStrict([]byte(attestation.EnvelopeJSON), &envelope); err != nil {
		return VerifiedStatement{}, invalid(err)
	}
	if envelope.PayloadType != PayloadType || len(envelope.Signatures) != 1 {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	if attestation.Issuer != policy.Issuer ||
		attestation.KeyID != envelope.Signatures[0].KeyID ||
		!sha256Pattern.MatchString(attestation.StatementDigest) {
		return VerifiedStatement{}, ErrInvalidAttestation
	}

	verifiers := make([]dsse.Verifier, 0, len(policy.Keys))
	for _, key := range policy.Keys {
		verifier, err := verifierFromKey(key)
		if err == nil {
			verifiers = append(verifiers, verifier)
		}
	}
	if len(verifiers) == 0 {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	envelopeVerifier, err := dsse.NewEnvelopeVerifier(verifiers...)
	if err != nil {
		return VerifiedStatement{}, invalid(err)
	}
	_, payload, err := envelopeVerifier.VerifyAndDecode(ctx, &envelope)
	if err != nil {
		return VerifiedStatement{}, invalid(err)
	}
	if !equalCanonical(payload) || digestHex(payload) != attestation.StatementDigest {
		return VerifiedStatement{}, ErrInvalidAttestation
	}

	var value statement
	if err := decodeStrict(payload, &value); err != nil {
		return VerifiedStatement{}, invalid(err)
	}
	if value.Type != StatementType || value.PredicateType != PredicateType ||
		len(value.Subject) != 1 || len(value.Subject[0].Digest) != 1 {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	subject := value.Subject[0]
	predicate := value.Predicate
	if predicate.AttestationID == "" ||
		predicate.AttestationID != attestation.AttestationID ||
		predicate.Publisher.Subject != attestation.PublisherSubject ||
		predicate.Issuer != policy.Issuer ||
		predicate.Purpose != "publish" ||
		predicate.Audience != policy.Audience ||
		predicate.ConsumerInstance != policy.ConsumerInstance ||
		predicate.Namespace != policy.Namespace ||
		subject.Name != policy.Artifact.Name ||
		subject.URI != policy.Artifact.URI ||
		subject.MediaType != policy.Artifact.MediaType ||
		subject.Digest["sha256"] != policy.Artifact.SHA256 ||
		predicate.Artifact.Kind != policy.Artifact.Kind ||
		predicate.Artifact.Identity != policy.Artifact.Identity ||
		predicate.Artifact.Version != policy.Artifact.Version {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, predicate.IssuedAt)
	if err != nil || predicate.Publisher.Subject == "" {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	if !attestation.IssuedAt.IsZero() && !issuedAt.Equal(attestation.IssuedAt) {
		return VerifiedStatement{}, ErrInvalidAttestation
	}
	return VerifiedStatement{
		AttestationID:    predicate.AttestationID,
		PublisherSubject: predicate.Publisher.Subject,
		IssuedAt:         issuedAt,
		StatementDigest:  attestation.StatementDigest,
	}, nil
}

func decodeStrict(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func invalid(err error) error {
	if err == nil {
		return ErrInvalidAttestation
	}
	return fmt.Errorf("%w: %v", ErrInvalidAttestation, err)
}
