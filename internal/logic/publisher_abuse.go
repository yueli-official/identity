package logic

import (
	"context"

	"github.com/yueli-official/foundation/go/abuse"

	"platform/services/identity/internal/identityabuse"
	"platform/services/identity/internal/iderr"
)

// AdmitPublisherAttestation protects signing operations while letting an exact
// idempotent retry reach the publisher module and recover its persisted result.
func (s *Service) AdmitPublisherAttestation(
	ctx context.Context,
	attemptID, ip, publisherSubject string,
) error {
	network, err := identityabuse.NetworkPrefix(ip)
	if err != nil {
		return iderr.AbuseUnavailable()
	}
	admission, err := s.abuse.PublisherIssue.Admit(ctx, abuse.Input{
		ID: abuse.AttemptID(attemptID),
		Signals: abuse.Signals{
			Network: network,
			Target:  publisherSubject,
		},
	})
	if err != nil {
		return iderr.AbuseUnavailable()
	}
	if admission.Disposition != abuse.DispositionAllow {
		return iderr.AccountLocked()
	}
	return nil
}
