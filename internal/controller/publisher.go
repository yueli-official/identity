package controller

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/publisher"
)

const publisherRecentAuthentication = 10 * time.Minute

type PublisherController struct {
	service *logic.Service
	module  *publisher.Module
	keys    publisher.KeyProvider
	trust   *publisher.TrustState
}

func NewPublisher(
	service *logic.Service,
	module *publisher.Module,
	keys publisher.KeyProvider,
	trust *publisher.TrustState,
) *PublisherController {
	return &PublisherController{service: service, module: module, keys: keys, trust: trust}
}

func (controller *PublisherController) IssuePublisherAttestation(
	ctx context.Context,
	req *v1.IssuePublisherAttestationReq,
) (*v1.IssuePublisherAttestationRes, error) {
	if controller.module == nil {
		return nil, iderr.PublisherSigningUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	_, identity, err := controller.service.RequireAuthentication(
		ctx,
		request.Cookie.Get(sessionCookie, "").String(),
		authentication.Requirement{
			FreshWithin:     publisherRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := controller.service.AdmitPublisherAttestation(
		ctx, req.IdempotencyKey, request.GetClientIp(), identity.ID,
	); err != nil {
		return nil, err
	}
	issued, err := controller.module.Issue(ctx, identity.ID, publisher.IssueCommand{
		IdempotencyKey: req.IdempotencyKey,
		Audience:       req.Audience, ConsumerInstance: req.ConsumerInstance,
		Namespace: req.Namespace,
		Artifact: publisher.Artifact{
			Kind: req.Artifact.Kind, Identity: req.Artifact.Identity,
			Version: req.Artifact.Version, Name: req.Artifact.Name,
			URI: req.Artifact.URI, MediaType: req.Artifact.MediaType,
			SHA256: req.Artifact.SHA256,
		},
	})
	if err != nil {
		return nil, mapPublisherError(err)
	}
	controller.service.RecordPublisherAttestation(ctx, identity.ID, issued)
	return &v1.IssuePublisherAttestationRes{
		AttestationID: issued.AttestationID, Issuer: issued.Issuer,
		PublisherSubject: issued.PublisherSubject,
		StatementDigest:  issued.StatementDigest, KeyID: issued.KeyID,
		Envelope: json.RawMessage(issued.EnvelopeJSON),
		IssuedAt: issued.IssuedAt.Format(time.RFC3339Nano),
	}, nil
}

func (controller *PublisherController) PublisherVerificationKeys(
	context.Context,
	*v1.PublisherVerificationKeysReq,
) (*v1.PublisherVerificationKeysRes, error) {
	if controller.keys == nil {
		return nil, iderr.PublisherSigningUnavailable()
	}
	keys := controller.keys.VerificationKeys()
	manifestVersion := 1
	if controller.trust != nil {
		current := controller.trust.Current()
		keys = current.Manifest.Keys
		manifestVersion = int(current.Manifest.ManifestVersion)
	}
	entries := make([]v1.PublisherVerificationKey, len(keys))
	for index, key := range keys {
		retiredAt := ""
		if key.RetiredAt != nil {
			retiredAt = key.RetiredAt.Format(time.RFC3339Nano)
		}
		entries[index] = v1.PublisherVerificationKey{
			KeyID: key.KeyID, Algorithm: key.Algorithm, Purpose: key.Purpose,
			Status: key.Status, PublicJWK: key.PublicJWK,
			ActivatedAt: key.ActivatedAt.Format(time.RFC3339Nano),
			RetiredAt:   retiredAt,
		}
	}
	return &v1.PublisherVerificationKeysRes{
		ManifestVersion: manifestVersion, Keys: entries,
	}, nil
}

func (controller *PublisherController) PublisherTrustManifest(
	context.Context,
	*v1.PublisherTrustManifestReq,
) (*v1.PublisherTrustManifestRes, error) {
	if controller.trust == nil {
		return nil, iderr.PublisherSigningUnavailable()
	}
	current := controller.trust.Current()
	manifest := current.Manifest
	keys := make([]v1.PublisherTrustManifestKey, len(manifest.Keys))
	for index, key := range manifest.Keys {
		keys[index] = v1.PublisherTrustManifestKey{
			KeyID: key.KeyID, Algorithm: key.Algorithm, Purpose: key.Purpose,
			Status: key.Status, PublicJWK: key.PublicJWK,
			ValidFrom:        key.ActivatedAt.Format(time.RFC3339Nano),
			ValidUntil:       optionalPublisherTime(key.ValidUntil),
			RetiredAt:        optionalPublisherTime(key.RetiredAt),
			CompromisedAt:    optionalPublisherTime(key.CompromisedAt),
			RevokedAt:        optionalPublisherTime(key.RevokedAt),
			RevocationReason: key.RevocationReason,
		}
	}
	return &v1.PublisherTrustManifestRes{
		Schema: manifest.Schema, ManifestVersion: manifest.ManifestVersion,
		Issuer: manifest.Issuer, IssuedAt: manifest.IssuedAt.Format(time.RFC3339Nano),
		PolicyVersion: manifest.PolicyVersion, RootKeyID: manifest.RootKeyID,
		Keys: keys, ManifestSignature: manifest.Signature,
		SnapshotHash: current.SnapshotHash,
	}, nil
}

func optionalPublisherTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func publisherTrustKeyResponses(keys []publisher.VerificationKey) []v1.PublisherTrustManifestKey {
	result := make([]v1.PublisherTrustManifestKey, len(keys))
	for index := range keys {
		result[index] = publisherTrustKeyResponse(keys[index])
	}
	return result
}

func publisherTrustKeyResponse(key publisher.VerificationKey) v1.PublisherTrustManifestKey {
	return v1.PublisherTrustManifestKey{
		KeyID: key.KeyID, Algorithm: key.Algorithm, Purpose: key.Purpose,
		Status: key.Status, PublicJWK: key.PublicJWK,
		ValidFrom:        key.ActivatedAt.Format(time.RFC3339Nano),
		ValidUntil:       optionalPublisherTime(key.ValidUntil),
		RetiredAt:        optionalPublisherTime(key.RetiredAt),
		CompromisedAt:    optionalPublisherTime(key.CompromisedAt),
		RevokedAt:        optionalPublisherTime(key.RevokedAt),
		RevocationReason: key.RevocationReason,
	}
}

func mapPublisherError(err error) error {
	switch {
	case errors.Is(err, publisher.ErrConsumerNotFound):
		return iderr.PublisherConsumerNotFound()
	case errors.Is(err, publisher.ErrConsumerDisabled):
		return iderr.PublisherConsumerDisabled()
	case errors.Is(err, publisher.ErrIdempotencyConflict):
		return iderr.PublisherIdempotencyConflict()
	case errors.Is(err, publisher.ErrInvalidCommand):
		return iderr.PublisherAttestationInvalid(iderr.PublisherAttestationReasonCommand)
	case errors.Is(err, publisher.ErrInvalidAttestation):
		return iderr.PublisherAttestationInvalid(iderr.PublisherAttestationReasonAttestation)
	case errors.Is(err, publisher.ErrSigningUnavailable):
		return iderr.PublisherSigningUnavailable()
	case errors.Is(err, publisher.ErrRotationPending):
		return iderr.PublisherRotationPending()
	case errors.Is(err, publisher.ErrInvalidKeyTransition):
		return iderr.PublisherKeyTransitionInvalid()
	case errors.Is(err, publisher.ErrInvalidTrustManifest):
		return iderr.PublisherTrustManifestInvalid()
	case errors.Is(err, publisher.ErrUntrustedRoot):
		return iderr.PublisherRootUntrusted()
	default:
		return err
	}
}
