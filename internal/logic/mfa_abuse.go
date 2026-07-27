package logic

import (
	"context"

	"github.com/yueli-official/foundation/go/abuse"

	"github.com/yueli-official/identity/internal/identityabuse"
	"github.com/yueli-official/identity/internal/iderr"
)

func (s *Service) AdmitMFAVerification(
	ctx context.Context,
	attemptID, ip, transactionID string,
) (abuse.Receipt, error) {
	network, err := identityabuse.NetworkPrefix(ip)
	if err != nil {
		return abuse.Receipt{}, iderr.AbuseUnavailable()
	}
	admission, err := identityabuse.Admit(
		ctx, s.abuse.MFAVerification, attemptID,
		network, transactionID, "",
	)
	if err != nil {
		return abuse.Receipt{}, iderr.AbuseUnavailable()
	}
	if admission.Disposition != abuse.DispositionAllow || admission.Replay {
		return abuse.Receipt{}, iderr.AccountLockedUntil(admission.RetryAt)
	}
	return admission.Receipt, nil
}

func (s *Service) ResolveMFAVerification(
	ctx context.Context,
	receipt abuse.Receipt,
	verified bool,
) {
	outcome := abuse.OutcomeKey("verification_rejected")
	if verified {
		outcome = "verified"
	}
	_ = s.abuse.MFAVerification.Resolve(ctx, receipt, outcome)
}

func (s *Service) AbortMFAVerification(
	ctx context.Context,
	receipt abuse.Receipt,
) {
	_ = s.abuse.MFAVerification.Resolve(ctx, receipt, "verification_aborted")
}
