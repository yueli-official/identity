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
}

func NewPublisher(
	service *logic.Service,
	module *publisher.Module,
	keys publisher.KeyProvider,
) *PublisherController {
	return &PublisherController{service: service, module: module, keys: keys}
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
	return &v1.PublisherVerificationKeysRes{ManifestVersion: 1, Keys: entries}, nil
}

func mapPublisherError(err error) error {
	switch {
	case errors.Is(err, publisher.ErrConsumerNotFound):
		return iderr.PublisherConsumerNotFound()
	case errors.Is(err, publisher.ErrConsumerDisabled):
		return iderr.PublisherConsumerDisabled()
	case errors.Is(err, publisher.ErrIdempotencyConflict):
		return iderr.PublisherIdempotencyConflict()
	case errors.Is(err, publisher.ErrInvalidCommand),
		errors.Is(err, publisher.ErrInvalidAttestation):
		return iderr.PublisherAttestationInvalid()
	case errors.Is(err, publisher.ErrSigningUnavailable):
		return iderr.PublisherSigningUnavailable()
	default:
		return err
	}
}
