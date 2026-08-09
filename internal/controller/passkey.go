package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/yueli-official/foundation/go/identifier"
	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/iderr"
)

const passkeyBindingRecentAuthentication = 10 * time.Minute

func (c *Controller) PasskeyLoginBegin(
	ctx context.Context,
	_ *v1.PasskeyLoginBeginReq,
) (*v1.PasskeyLoginBeginRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	if err := c.svc.AdmitPasskeyCeremony(
		ctx, identifier.MustNew().String(), request.GetClientIp(),
	); err != nil {
		return nil, err
	}
	result, err := c.authn.BeginPasskeyAuthentication(ctx)
	if err != nil {
		return nil, mapPasskeyError(err)
	}
	return &v1.PasskeyLoginBeginRes{
		CeremonyID: result.CeremonyID,
		ExpiresAt:  result.ExpiresAt.Format(time.RFC3339),
		Options:    result.Options,
	}, nil
}

func (c *Controller) PasskeyLoginFinish(
	ctx context.Context,
	req *v1.PasskeyLoginFinishReq,
) (*v1.PasskeyLoginFinishRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	result, err := c.authn.FinishPasskeyAuthentication(
		ctx,
		authentication.FinishPasskeyAuthenticationRequest{
			CeremonyID: req.CeremonyID, Response: req.Response,
			UserAgent: request.UserAgent(), IP: request.GetClientIp(),
		},
	)
	if err != nil {
		return nil, mapPasskeyError(err)
	}
	identity, err := c.svc.GetByID(ctx, result.IdentityID)
	if err != nil {
		return nil, err
	}
	c.setSessionCookie(request, result.SessionID)
	return &v1.PasskeyLoginFinishRes{UserKey: identity.UserKey, Email: identity.Email}, nil
}

func (c *Controller) PasskeyRegistrationBegin(
	ctx context.Context,
	_ *v1.PasskeyRegistrationBeginReq,
) (*v1.PasskeyRegistrationBeginRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	_, identity, err := c.svc.RequireAuthentication(
		ctx, sessionID,
		authentication.Requirement{
			FreshWithin:     passkeyBindingRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	profile, _ := c.svc.GetProfile(ctx, identity.ID)
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = identity.Email
	}
	result, err := c.authn.BeginPasskeyRegistration(
		ctx,
		authentication.BeginPasskeyRegistrationRequest{
			IdentityID: identity.ID, SessionID: sessionID,
			Name: identity.Email, DisplayName: displayName,
		},
	)
	if err != nil {
		return nil, mapPasskeyError(err)
	}
	return &v1.PasskeyRegistrationBeginRes{
		CeremonyID: result.CeremonyID,
		ExpiresAt:  result.ExpiresAt.Format(time.RFC3339),
		Options:    result.Options,
	}, nil
}

func (c *Controller) PasskeyRegistrationFinish(
	ctx context.Context,
	req *v1.PasskeyRegistrationFinishReq,
) (*v1.PasskeyRegistrationFinishRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	if _, _, err := c.svc.RequireAuthentication(
		ctx,
		sessionID,
		authentication.Requirement{
			FreshWithin:     passkeyBindingRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	); err != nil {
		return nil, err
	}
	credential, err := c.authn.FinishPasskeyRegistration(
		ctx,
		authentication.FinishPasskeyRegistrationRequest{
			CeremonyID: req.CeremonyID, SessionID: sessionID,
			Label: strings.TrimSpace(req.Label), Response: req.Response,
		},
	)
	if err != nil {
		return nil, mapPasskeyError(err)
	}
	return &v1.PasskeyRegistrationFinishRes{Passkey: passkeyEntry(credential)}, nil
}

func (c *Controller) ListPasskeys(
	ctx context.Context,
	_ *v1.ListPasskeysReq,
) (*v1.ListPasskeysRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	identity, err := c.svc.Me(ctx, request.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	credentials, err := c.authn.ListPasskeys(ctx, identity.ID)
	if err != nil {
		return nil, err
	}
	entries := make([]v1.PasskeyEntry, len(credentials))
	for i := range credentials {
		entries[i] = passkeyEntry(credentials[i])
	}
	return &v1.ListPasskeysRes{Entries: entries}, nil
}

func (c *Controller) RenamePasskey(
	ctx context.Context,
	req *v1.RenamePasskeyReq,
) (*v1.RenamePasskeyRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	identity, err := c.svc.Me(ctx, request.Cookie.Get(sessionCookie, "").String())
	if err != nil {
		return nil, err
	}
	credential, err := c.authn.RenamePasskey(
		ctx, identity.ID, req.ID, strings.TrimSpace(req.Label),
	)
	if err != nil {
		return nil, mapPasskeyError(err)
	}
	return &v1.RenamePasskeyRes{Passkey: passkeyEntry(credential)}, nil
}

func (c *Controller) RevokePasskey(
	ctx context.Context,
	req *v1.RevokePasskeyReq,
) (*v1.RevokePasskeyRes, error) {
	if c.authn == nil {
		return nil, iderr.PasskeyUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	_, identity, err := c.svc.RequireAuthentication(
		ctx,
		request.Cookie.Get(sessionCookie, "").String(),
		authentication.Requirement{
			FreshWithin:     passkeyBindingRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := c.authn.RevokePasskey(ctx, identity.ID, req.ID); err != nil {
		return nil, mapPasskeyError(err)
	}
	return &v1.RevokePasskeyRes{}, nil
}

func passkeyEntry(value authentication.PasskeyCredential) v1.PasskeyEntry {
	lastUsedAt := ""
	if value.LastUsedAt != nil {
		lastUsedAt = value.LastUsedAt.Format(time.RFC3339)
	}
	transports := value.Transports
	if transports == nil {
		transports = []string{}
	}
	return v1.PasskeyEntry{
		ID: value.ID, Label: value.Label, Status: value.Status,
		Transports: transports, Attachment: value.Attachment,
		BackupEligible: value.BackupEligible, BackupState: value.BackupState,
		CreatedAt: value.CreatedAt.Format(time.RFC3339), LastUsedAt: lastUsedAt,
	}
}

func mapPasskeyError(err error) error {
	switch {
	case errors.Is(err, authentication.ErrPasskeyUnavailable):
		return iderr.PasskeyUnavailable()
	case errors.Is(err, authentication.ErrPasskeyExists):
		return iderr.PasskeyExists()
	case errors.Is(err, authentication.ErrLastAuthenticator):
		return iderr.LastCredential()
	case errors.Is(err, authentication.ErrCeremonyInvalid),
		errors.Is(err, authentication.ErrCeremonyExpired),
		errors.Is(err, authentication.ErrCeremonyConsumed),
		errors.Is(err, authentication.ErrPasskeyNotFound),
		errors.Is(err, authentication.ErrPasskeyConcurrentUse):
		return iderr.PasskeyCeremonyInvalid()
	default:
		return err
	}
}
