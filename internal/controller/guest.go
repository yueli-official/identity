package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	foundationauth "github.com/yueli-official/foundation/go/auth"
	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/guest"
	"github.com/yueli-official/identity/internal/iderr"
)

type Guest struct {
	service *guest.Service
}

func NewGuest(service *guest.Service) *Guest {
	return &Guest{service: service}
}

func (controller *Guest) GuestSessionCreate(ctx context.Context, req *v1.GuestSessionCreateReq) (*v1.GuestSessionCreateRes, error) {
	created, err := controller.service.Create(ctx, req.ClientID, time.Duration(req.RequestedTTLSeconds)*time.Second)
	if err != nil {
		return nil, guestError(err)
	}
	return &v1.GuestSessionCreateRes{
		SubjectID:           created.SubjectID,
		SessionToken:        created.SessionToken,
		EffectiveTTLSeconds: int64(created.EffectiveTTL / time.Second),
		ExpiresAt:           created.ExpiresAt,
	}, nil
}

func (controller *Guest) GuestToken(ctx context.Context, req *v1.GuestTokenReq) (*v1.GuestTokenRes, error) {
	issued, err := controller.service.Token(ctx, req.ClientID, req.SessionToken, req.Audience)
	if err != nil {
		return nil, guestError(err)
	}
	return &v1.GuestTokenRes{AccessToken: issued.AccessToken, ExpiresInSeconds: int64(issued.ExpiresIn / time.Second)}, nil
}

func (controller *Guest) GuestSessionClaim(ctx context.Context, req *v1.GuestSessionClaimReq) (*v1.GuestSessionClaimRes, error) {
	principal, ok := foundationauth.FromContext(ctx)
	subjectKind, _ := principal.Claim("subject_kind")
	if !ok || principal == nil || strings.TrimSpace(principal.Subject) == "" || subjectKind == "guest" {
		return nil, iderr.NotAuthenticated()
	}
	claimed, err := controller.service.ClaimForAudience(ctx, req.ClientID, req.SessionToken, principal.Subject, req.Audience)
	if err != nil {
		return nil, guestError(err)
	}
	return &v1.GuestSessionClaimRes{SubjectID: claimed.SubjectID, UserID: claimed.UserID, ClaimedAt: claimed.ClaimedAt, ClaimToken: claimed.ClaimToken}, nil
}

func guestError(err error) error {
	switch {
	case errors.Is(err, guest.ErrInvalidRequest):
		return iderr.InvalidGuestRequest()
	case errors.Is(err, guest.ErrInvalidSession):
		return iderr.InvalidGuestSession()
	case errors.Is(err, guest.ErrInvalidAudience):
		return iderr.InvalidGuestAudience()
	case errors.Is(err, guest.ErrClaimConflict):
		return iderr.GuestClaimConflict()
	default:
		return err
	}
}
