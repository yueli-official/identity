package controller

import (
	"context"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/publisher"
)

const (
	publisherKeyPrepareAction    = "identity.admin.publisher.key.prepare"
	publisherManifestApplyAction = "identity.admin.publisher.manifest.apply"
)

type PublisherAdminController struct {
	auth  *Controller
	svc   *logic.Service
	admin *publisher.KeyAdministration
}

func NewPublisherAdmin(
	auth *Controller,
	service *logic.Service,
	admin *publisher.KeyAdministration,
) *PublisherAdminController {
	return &PublisherAdminController{auth: auth, svc: service, admin: admin}
}

func (controller *PublisherAdminController) AdminPreparePublisherKey(
	ctx context.Context,
	_ *v1.AdminPreparePublisherKeyReq,
) (*v1.AdminPreparePublisherKeyRes, error) {
	if controller.admin == nil {
		return nil, iderr.PublisherSigningUnavailable()
	}
	adminID, err := controller.auth.requireAdminAction(
		ctx, publisherKeyPrepareAction, "publisher:key-ring",
	)
	if err != nil {
		return nil, err
	}
	prepared, keys, err := controller.admin.PrepareRotation(ctx)
	if err != nil {
		return nil, mapPublisherError(err)
	}
	ctx = actor.WithIdentity(ctx, adminID)
	controller.svc.RecordPublisherKeyPrepared(ctx, adminID, prepared)
	return &v1.AdminPreparePublisherKeyRes{
		Prepared: publisherTrustKeyResponse(prepared),
		Keys:     publisherTrustKeyResponses(keys),
	}, nil
}

func (controller *PublisherAdminController) AdminApplyPublisherTrustManifest(
	ctx context.Context,
	req *v1.AdminApplyPublisherTrustManifestReq,
) (*v1.AdminApplyPublisherTrustManifestRes, error) {
	if controller.admin == nil {
		return nil, iderr.PublisherSigningUnavailable()
	}
	resource, err := publisher.TrustManifestResource(req.Manifest)
	if err != nil {
		return nil, mapPublisherError(err)
	}
	adminID, err := controller.auth.requireAdminAction(
		ctx, publisherManifestApplyAction, resource,
	)
	if err != nil {
		return nil, err
	}
	verified, err := controller.admin.ApplyTrustManifest(ctx, req.Manifest)
	if err != nil {
		return nil, mapPublisherError(err)
	}
	activeKeyID := ""
	for _, key := range verified.Manifest.Keys {
		if key.Status == publisher.KeyStatusActive {
			activeKeyID = key.KeyID
			break
		}
	}
	ctx = actor.WithIdentity(ctx, adminID)
	controller.svc.RecordPublisherManifestApplied(
		ctx, adminID, verified, activeKeyID,
	)
	return &v1.AdminApplyPublisherTrustManifestRes{
		ManifestVersion: verified.Manifest.ManifestVersion,
		SnapshotHash:    verified.SnapshotHash,
		ActiveKeyID:     activeKeyID,
		SigningEnabled:  activeKeyID != "",
	}, nil
}
