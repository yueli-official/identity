package publisher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/yueli-official/foundation/go/identifier"
)

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	tokenPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,255}$`)
)

type Module struct {
	issuer    string
	consumers map[string]Consumer
	store     Store
	signer    KeyProvider
	now       func() time.Time
}

func New(config Config) (*Module, error) {
	if strings.TrimSpace(config.Issuer) == "" || config.Store == nil || config.Signer == nil {
		return nil, fmt.Errorf("%w: issuer, store, and signer are required", ErrInvalidCommand)
	}
	consumers := make(map[string]Consumer, len(config.Consumers))
	for _, consumer := range config.Consumers {
		if consumer.Audience == "" || consumer.Instance == "" {
			return nil, fmt.Errorf("%w: consumer audience and instance are required", ErrInvalidCommand)
		}
		consumers[consumer.Audience+"\x00"+consumer.Instance] = consumer
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Module{
		issuer: strings.TrimSpace(config.Issuer), consumers: consumers,
		store: config.Store, signer: config.Signer, now: now,
	}, nil
}

func (module *Module) Issue(
	ctx context.Context,
	publisherSubject string,
	command IssueCommand,
) (Attestation, error) {
	publisherSubject = strings.TrimSpace(publisherSubject)
	consumer, err := module.validateCommand(publisherSubject, command)
	if err != nil {
		return Attestation{}, err
	}
	commandBytes, err := canonicalJSON(command)
	if err != nil {
		return Attestation{}, fmt.Errorf("%w: canonical command: %v", ErrInvalidCommand, err)
	}
	commandDigest := digestHex(commandBytes)
	if existing, ok, err := module.store.GetByIdempotency(
		ctx, module.issuer, publisherSubject, command.IdempotencyKey,
	); err != nil {
		return Attestation{}, err
	} else if ok {
		if existing.CommandDigest != commandDigest {
			return Attestation{}, ErrIdempotencyConflict
		}
		return existing, nil
	}

	issuedAt := module.now().UTC()
	attestationID := identifier.MustNew().String()
	value := statement{
		Type: StatementType,
		Subject: []subject{{
			Name: command.Artifact.Name, URI: command.Artifact.URI,
			MediaType: command.Artifact.MediaType,
			Digest:    map[string]string{"sha256": command.Artifact.SHA256},
		}},
		PredicateType: PredicateType,
		Predicate: predicate{
			AttestationID: attestationID, Issuer: module.issuer, Purpose: "publish",
			Publisher: publisherIdentity{Subject: publisherSubject},
			Audience:  command.Audience, ConsumerInstance: command.ConsumerInstance,
			Namespace: command.Namespace,
			Artifact: artifactIdentity{
				Kind: command.Artifact.Kind, Identity: command.Artifact.Identity,
				Version: command.Artifact.Version,
			},
			IssuedAt: issuedAt.Format(time.RFC3339Nano),
		},
	}
	payload, err := canonicalJSON(value)
	if err != nil {
		return Attestation{}, fmt.Errorf("%w: canonical statement: %v", ErrInvalidCommand, err)
	}
	envelopeSigner, err := dsse.NewEnvelopeSigner(module.signer)
	if err != nil {
		return Attestation{}, fmt.Errorf("%w: %v", ErrSigningUnavailable, err)
	}
	envelope, err := envelopeSigner.SignPayload(ctx, PayloadType, payload)
	if err != nil {
		return Attestation{}, fmt.Errorf("%w: %v", ErrSigningUnavailable, err)
	}
	if len(envelope.Signatures) != 1 {
		return Attestation{}, ErrSigningUnavailable
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return Attestation{}, err
	}
	record := Attestation{
		AttestationID: attestationID, Issuer: module.issuer,
		PublisherSubject: publisherSubject, StatementDigest: digestHex(payload),
		CommandDigest: commandDigest, KeyID: envelope.Signatures[0].KeyID,
		EnvelopeJSON: string(envelopeJSON), IssuedAt: issuedAt,
		IdempotencyKey: command.IdempotencyKey, Audience: consumer.Audience,
		ConsumerInstance: command.ConsumerInstance, Namespace: command.Namespace,
		Artifact: command.Artifact,
	}
	stored, inserted, err := module.store.PutIfAbsent(ctx, record)
	if err != nil {
		return Attestation{}, err
	}
	if !inserted && stored.CommandDigest != commandDigest {
		return Attestation{}, ErrIdempotencyConflict
	}
	return stored, nil
}

func (module *Module) validateCommand(subject string, command IssueCommand) (Consumer, error) {
	if subject == "" || len(command.IdempotencyKey) < 16 || len(command.IdempotencyKey) > 200 {
		return Consumer{}, ErrInvalidCommand
	}
	consumer, ok := module.consumers[command.Audience+"\x00"+command.ConsumerInstance]
	if !ok {
		return Consumer{}, ErrConsumerNotFound
	}
	if consumer.Disabled {
		return Consumer{}, ErrConsumerDisabled
	}
	policy, ok := consumer.ArtifactKinds[command.Artifact.Kind]
	if !ok || policy.MediaType != command.Artifact.MediaType {
		return Consumer{}, ErrInvalidCommand
	}
	if !validStableURI(command.Audience) || !validStableURI(command.ConsumerInstance) ||
		!tokenPattern.MatchString(command.Namespace) ||
		!tokenPattern.MatchString(command.Artifact.Kind) ||
		!tokenPattern.MatchString(command.Artifact.Identity) ||
		!tokenPattern.MatchString(command.Artifact.Version) ||
		strings.TrimSpace(command.Artifact.Name) == "" ||
		!validStableURI(command.Artifact.URI) ||
		!sha256Pattern.MatchString(command.Artifact.SHA256) {
		return Consumer{}, ErrInvalidCommand
	}
	return consumer, nil
}

func validStableURI(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Scheme != "" && !strings.ContainsAny(value, " \t\r\n")
}

func digestHex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func equalCanonical(raw []byte) bool {
	canonical, err := canonicalJSON(json.RawMessage(raw))
	return err == nil && bytes.Equal(raw, canonical)
}
