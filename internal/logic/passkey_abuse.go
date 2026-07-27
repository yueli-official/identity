package logic

import (
	"context"

	"github.com/yueli-official/foundation/go/abuse"

	"github.com/yueli-official/identity/internal/identityabuse"
	"github.com/yueli-official/identity/internal/iderr"
)

// AdmitPasskeyCeremony protects the anonymous ceremony-begin path from
// database/challenge amplification. A ceremony still has its own expiry,
// one-time consumption and per-ceremony failure budget.
func (s *Service) AdmitPasskeyCeremony(
	ctx context.Context,
	attemptID, ip string,
) error {
	network, err := identityabuse.NetworkPrefix(ip)
	if err != nil {
		return iderr.AbuseUnavailable()
	}
	admission, err := s.abuse.PasskeyCeremony.Admit(ctx, abuse.Input{
		ID:      abuse.AttemptID(attemptID),
		Signals: abuse.Signals{Network: network},
	})
	if err != nil {
		return iderr.AbuseUnavailable()
	}
	if admission.Disposition != abuse.DispositionAllow || admission.Replay {
		return iderr.AccountLockedUntil(admission.RetryAt)
	}
	return nil
}
