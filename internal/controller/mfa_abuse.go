package controller

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/yueli-official/foundation/go/abuse"
	"github.com/yueli-official/foundation/go/identifier"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/logic"
)

func admitMFAVerification(
	ctx context.Context,
	service *logic.Service,
	target string,
) (abuse.Receipt, error) {
	request := ghttp.RequestFromCtx(ctx)
	return service.AdmitMFAVerification(
		ctx, identifier.MustNew().String(), request.GetClientIp(), target,
	)
}

func resolveMFAVerification(
	ctx context.Context,
	service *logic.Service,
	receipt abuse.Receipt,
	err error,
) {
	switch {
	case err == nil:
		service.ResolveMFAVerification(ctx, receipt, true)
	case isRejectedMFAVerification(err):
		service.ResolveMFAVerification(ctx, receipt, false)
	default:
		service.AbortMFAVerification(ctx, receipt)
	}
}

func isRejectedMFAVerification(err error) bool {
	return errors.Is(err, authentication.ErrTOTPCodeInvalid) ||
		errors.Is(err, authentication.ErrTOTPEnrollmentInvalid) ||
		errors.Is(err, authentication.ErrAuthenticationTransactionInvalid) ||
		errors.Is(err, authentication.ErrRecoveryCodeInvalid) ||
		errors.Is(err, authentication.ErrStepUpRequestInvalid)
}
