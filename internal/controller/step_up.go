package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/oidc"
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
	requirement, ok := stepUpRequirement(req.Requirement)
	if !ok {
		return nil, iderr.StepUpRequestInvalid()
	}
	result, err := controller.authn.BeginStepUp(
		ctx, authentication.BeginStepUpRequest{
			IdentityID: identity.ID, SessionID: session.ID,
			Audience: req.Audience, Action: req.Action, Resource: req.Resource,
			Requirement: requirement,
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
	receipt, err := admitMFAVerification(ctx, controller.svc, req.TransactionID)
	if err != nil {
		return nil, err
	}
	material, err := controller.authn.FinishTOTPAction(
		ctx, authentication.FinishTOTPActionRequest{
			TransactionID: req.TransactionID, Session: session,
			Code: strings.TrimSpace(req.Code),
		},
	)
	resolveMFAVerification(ctx, controller.svc, receipt, err)
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

func stepUpRequirement(value v1.StepUpRequirement) (authentication.Requirement, bool) {
	level := authentication.Level(strings.TrimSpace(value.MinimumLevel))
	switch level {
	case "", authentication.LevelAAL1, authentication.LevelAAL2, authentication.LevelAAL3:
	default:
		return authentication.Requirement{}, false
	}
	profile := authentication.Profile(strings.TrimSpace(value.MinimumProfile))
	switch profile {
	case "", authentication.ProfileBaseline, authentication.ProfileMultiFactor,
		authentication.ProfilePhishingResistant:
	default:
		return authentication.Requirement{}, false
	}
	return authentication.Requirement{
		FreshWithin:        time.Duration(value.FreshWithinSeconds) * time.Second,
		MinimumLevel:       level,
		MinimumProfile:     profile,
		UserVerification:   value.UserVerification,
		PhishingResistant:  value.PhishingResistant,
		MinimumFactorCount: value.MinimumFactorCount,
		RecoveryAllowed:    false,
	}, true
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
