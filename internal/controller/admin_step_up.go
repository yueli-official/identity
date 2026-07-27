package controller

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"platform/services/identity/internal/iderr"
	"platform/services/identity/stepup"
)

const stepUpProofHeader = "X-Step-Up-Proof"

func (c *Controller) requireAdminAction(
	ctx context.Context,
	action, resource string,
) (string, error) {
	adminID, err := c.requireAdmin(ctx)
	if err != nil {
		return "", err
	}
	// Hermetic/controller tests may construct the lightweight controller
	// without the runtime verifier. Production wiring always supplies it.
	if c.adminStepUp == nil {
		return adminID, nil
	}
	raw := strings.TrimSpace(ghttp.RequestFromCtx(ctx).Header.Get(stepUpProofHeader))
	if raw == "" {
		return "", iderr.StepUpRequired([]string{"action_bound_proof"})
	}
	evidence, err := c.adminStepUp.VerifyAndConsume(ctx, raw, action, resource)
	if err != nil {
		if errors.Is(err, stepup.ErrReplay) {
			return "", iderr.StepUpProofReplayed()
		}
		return "", iderr.StepUpProofInvalid()
	}
	if evidence.Subject != adminID {
		return "", iderr.StepUpProofInvalid()
	}
	return adminID, nil
}

func adminRoleResource(identityID, role string) string {
	return "identity:" + identityID + ":role:" + strings.TrimSpace(role)
}

func adminStatusResource(identityID, status string) string {
	return "identity:" + identityID + ":status:" + strings.TrimSpace(status)
}

func adminIdentityResource(identityID string) string {
	return "identity:" + identityID
}

func adminCreateResource(email string, roles []string) string {
	canonicalRoles := append([]string{}, roles...)
	sort.Strings(canonicalRoles)
	return "identity:new:" + strings.ToLower(strings.TrimSpace(email)) +
		":roles:" + strings.Join(canonicalRoles, ",")
}
