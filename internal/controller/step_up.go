package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
)

type StepUpProofSigner interface {
	MintStepUpProof(oidc.StepUpProofInput) (string, error)
}

type StepUpController struct {
	svc    *logic.Service
	authn  *authentication.Module
	signer StepUpProofSigner
	issuer string
}

func NewStepUp(
	svc *logic.Service,
	authn *authentication.Module,
	signer StepUpProofSigner,
	issuer string,
) *StepUpController {
	return &StepUpController{svc: svc, authn: authn, signer: signer, issuer: issuer}
}

func (controller *StepUpController) StepUpBegin(
	ctx context.Context,
	req *v1.StepUpBeginReq,
) (*v1.StepUpBeginRes, error) {
	if controller.authn == nil || controller.signer == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	session, identity, err := controller.svc.AuthenticatedSession(
		ctx, request.Cookie.Get(sessionCookie, "").String(),
	)
	if err != nil {
		return nil, err
	}
	result, err := controller.authn.BeginStepUp(
		ctx, authentication.BeginStepUpRequest{
			IdentityID: identity.ID, SessionID: session.ID,
			Audience: req.Audience, Action: req.Action, Resource: req.Resource,
			Requirement: stepUpRequirement(req.Requirement),
			Context:     session.Authentication,
		},
	)
	if err != nil {
		return nil, mapStepUpError(err)
	}
	response := &v1.StepUpBeginRes{
		Satisfied: result.Satisfied, TransactionID: result.TransactionID,
		Methods: result.Methods,
	}
	if !result.ExpiresAt.IsZero() {
		response.ExpiresAt = result.ExpiresAt.Format(time.RFC3339)
	}
	if result.Satisfied {
		response.Proof, err = controller.sign(result.Proof)
		if err != nil {
			return nil, err
		}
	}
	return response, nil
}

func (controller *StepUpController) StepUpTOTPFinish(
	ctx context.Context,
	req *v1.StepUpTOTPFinishReq,
) (*v1.StepUpTOTPFinishRes, error) {
	if controller.authn == nil || controller.signer == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	session, _, err := controller.svc.AuthenticatedSession(
		ctx, request.Cookie.Get(sessionCookie, "").String(),
	)
	if err != nil {
		return nil, err
	}
	material, err := controller.authn.FinishTOTPAction(
		ctx, authentication.FinishTOTPActionRequest{
			TransactionID: req.TransactionID, SessionID: session.ID,
			Context: session.Authentication, Code: strings.TrimSpace(req.Code),
		},
	)
	if err != nil {
		return nil, mapStepUpError(err)
	}
	proof, err := controller.sign(material)
	if err != nil {
		return nil, err
	}
	return &v1.StepUpTOTPFinishRes{Proof: proof}, nil
}

func (controller *StepUpController) sign(
	material authentication.StepUpProofMaterial,
) (string, error) {
	return controller.signer.MintStepUpProof(oidc.StepUpProofInput{
		Issuer: controller.issuer, ID: material.ID,
		Subject: material.IdentityID, SessionID: material.SessionID,
		Audience: material.Audience, Action: material.Action,
		ResourceDigest: material.ResourceDigest,
		Authentication: material.Authentication, IssuedAt: material.IssuedAt,
		TTL: 2 * time.Minute,
	})
}

func stepUpRequirement(value v1.StepUpRequirement) authentication.Requirement {
	return authentication.Requirement{
		FreshWithin:        time.Duration(value.FreshWithinSeconds) * time.Second,
		MinimumLevel:       authentication.Level(value.MinimumLevel),
		MinimumProfile:     authentication.Profile(value.MinimumProfile),
		UserVerification:   value.UserVerification,
		PhishingResistant:  value.PhishingResistant,
		MinimumFactorCount: value.MinimumFactorCount,
		RecoveryAllowed:    false,
	}
}

func mapStepUpError(err error) error {
	switch {
	case errors.Is(err, authentication.ErrStepUpRequestInvalid):
		return iderr.StepUpRequestInvalid()
	case errors.Is(err, authentication.ErrStepUpMethodUnavailable):
		return iderr.StepUpMethodUnavailable()
	default:
		return mapMFAError(err)
	}
}
