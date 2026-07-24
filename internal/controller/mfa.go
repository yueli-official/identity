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
)

const mfaSecurityRecentAuthentication = 10 * time.Minute

func (c *Controller) TOTPLogin(
	ctx context.Context,
	req *v1.TOTPLoginReq,
) (*v1.TOTPLoginRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	result, err := c.authn.FinishTOTPLogin(
		ctx, authentication.FinishTOTPLoginRequest{
			TransactionID: req.TransactionID, Code: strings.TrimSpace(req.Code),
		},
	)
	if err != nil {
		return nil, mapMFAError(err)
	}
	identity, err := c.svc.GetByID(ctx, result.IdentityID)
	if err != nil {
		return nil, err
	}
	c.setSessionCookie(ghttp.RequestFromCtx(ctx), result.SessionID)
	return &v1.TOTPLoginRes{ID: identity.ID, Email: identity.Email}, nil
}

func (c *Controller) RecoveryLogin(
	ctx context.Context,
	req *v1.RecoveryLoginReq,
) (*v1.RecoveryLoginRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	result, err := c.authn.FinishRecoveryLogin(
		ctx, authentication.FinishRecoveryLoginRequest{
			TransactionID: req.TransactionID, Code: strings.TrimSpace(req.Code),
		},
	)
	if err != nil {
		return nil, mapMFAError(err)
	}
	identity, err := c.svc.GetByID(ctx, result.IdentityID)
	if err != nil {
		return nil, err
	}
	c.setSessionCookie(ghttp.RequestFromCtx(ctx), result.SessionID)
	return &v1.RecoveryLoginRes{
		ID: identity.ID, Email: identity.Email, Restricted: true,
	}, nil
}

func (c *Controller) RecoverySession(
	ctx context.Context,
	_ *v1.RecoverySessionReq,
) (*v1.RecoverySessionRes, error) {
	request := ghttp.RequestFromCtx(ctx)
	session, _, err := c.svc.RequireAuthentication(
		ctx, request.Cookie.Get(sessionCookie, "").String(),
		authentication.Requirement{
			MinimumLevel: authentication.LevelAAL1, RecoveryAllowed: true,
		},
	)
	if err != nil {
		return nil, err
	}
	if !session.Authentication.Recovery {
		return nil, iderr.Forbidden()
	}
	return &v1.RecoverySessionRes{
		IdentityID: session.IdentityID, ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (c *Controller) TOTPEnrollmentBegin(
	ctx context.Context,
	req *v1.TOTPEnrollmentBeginReq,
) (*v1.TOTPEnrollmentBeginRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	_, identity, err := c.svc.RequireAuthentication(
		ctx, sessionID, authentication.Requirement{
			FreshWithin:  mfaSecurityRecentAuthentication,
			MinimumLevel: authentication.LevelAAL1, RecoveryAllowed: true,
		},
	)
	if err != nil {
		return nil, err
	}
	result, err := c.authn.BeginTOTPEnrollment(
		ctx, authentication.BeginTOTPEnrollmentRequest{
			IdentityID: identity.ID, SessionID: sessionID,
			AccountName: identity.Email, Label: strings.TrimSpace(req.Label),
		},
	)
	if err != nil {
		return nil, mapMFAError(err)
	}
	return &v1.TOTPEnrollmentBeginRes{
		AuthenticatorID: result.AuthenticatorID, URI: result.URI,
		Secret: result.Secret, ExpiresAt: result.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (c *Controller) TOTPEnrollmentFinish(
	ctx context.Context,
	req *v1.TOTPEnrollmentFinishReq,
) (*v1.TOTPEnrollmentFinishRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	_, identity, err := c.svc.RequireAuthentication(
		ctx, sessionID, authentication.Requirement{
			FreshWithin:  mfaSecurityRecentAuthentication,
			MinimumLevel: authentication.LevelAAL1, RecoveryAllowed: true,
		},
	)
	if err != nil {
		return nil, err
	}
	result, err := c.authn.FinishTOTPEnrollment(
		ctx, authentication.FinishTOTPEnrollmentRequest{
			IdentityID: identity.ID, SessionID: sessionID,
			AuthenticatorID: req.AuthenticatorID, Code: strings.TrimSpace(req.Code),
		},
	)
	if err != nil {
		return nil, mapMFAError(err)
	}
	return &v1.TOTPEnrollmentFinishRes{
		Authenticator: totpEntry(result.Authenticator),
		RecoveryCodes: result.RecoveryCodes,
	}, nil
}

func (c *Controller) ListTOTP(
	ctx context.Context,
	_ *v1.ListTOTPReq,
) (*v1.ListTOTPRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	identity, err := c.svc.Me(ctx, request.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	authenticators, err := c.authn.ListTOTP(ctx, identity.ID)
	if err != nil {
		return nil, mapMFAError(err)
	}
	entries := make([]v1.TOTPEntry, len(authenticators))
	for index := range authenticators {
		entries[index] = totpEntry(authenticators[index])
	}
	return &v1.ListTOTPRes{Entries: entries}, nil
}

func (c *Controller) RevokeTOTP(
	ctx context.Context,
	req *v1.RevokeTOTPReq,
) (*v1.RevokeTOTPRes, error) {
	if c.authn == nil {
		return nil, iderr.MFAUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	_, identity, err := c.svc.RequireAuthentication(
		ctx, request.Cookie.Get(sessionCookie, "").String(),
		authentication.Requirement{
			FreshWithin:  mfaSecurityRecentAuthentication,
			MinimumLevel: authentication.LevelAAL2, RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := c.authn.RevokeTOTP(ctx, identity.ID, req.ID); err != nil {
		return nil, mapMFAError(err)
	}
	return &v1.RevokeTOTPRes{}, nil
}

func totpEntry(value authentication.TOTPAuthenticator) v1.TOTPEntry {
	verifiedAt := ""
	if value.VerifiedAt != nil {
		verifiedAt = value.VerifiedAt.Format(time.RFC3339)
	}
	lastUsedAt := ""
	if value.LastUsedAt != nil {
		lastUsedAt = value.LastUsedAt.Format(time.RFC3339)
	}
	return v1.TOTPEntry{
		ID: value.ID, Label: value.Label, Status: value.Status,
		CreatedAt:  value.CreatedAt.Format(time.RFC3339),
		VerifiedAt: verifiedAt, LastUsedAt: lastUsedAt,
	}
}

func mapMFAError(err error) error {
	switch {
	case errors.Is(err, authentication.ErrMFAUnavailable):
		return iderr.MFAUnavailable()
	case errors.Is(err, authentication.ErrTOTPEnrollmentInvalid):
		return iderr.TOTPEnrollmentInvalid()
	case errors.Is(err, authentication.ErrTOTPCodeInvalid):
		return iderr.TOTPCodeInvalid()
	case errors.Is(err, authentication.ErrTOTPNotFound):
		return iderr.TOTPNotFound()
	case errors.Is(err, authentication.ErrAuthenticationTransactionInvalid):
		return iderr.MFATransactionInvalid()
	case errors.Is(err, authentication.ErrRecoveryCodeInvalid):
		return iderr.RecoveryCodeInvalid()
	default:
		return err
	}
}
