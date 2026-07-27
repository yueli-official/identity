package controller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/githubbinding"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
)

const githubBindingRecentAuthentication = 10 * time.Minute

type GitHubBindingController struct {
	service       *logic.Service
	module        *githubbinding.Module
	webhookSecret []byte
	fallbackURL   string
}

func NewGitHubBinding(
	service *logic.Service,
	module *githubbinding.Module,
	webhookSecret []byte,
	fallbackURL string,
) *GitHubBindingController {
	return &GitHubBindingController{
		service: service, module: module, webhookSecret: webhookSecret,
		fallbackURL: safeReturnTo(fallbackURL),
	}
}

func (controller *GitHubBindingController) BeginGitHubBinding(
	ctx context.Context,
	req *v1.BeginGitHubBindingReq,
) (*v1.BeginGitHubBindingRes, error) {
	if controller.module == nil {
		return nil, iderr.GitHubBindingUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	_, identity, err := controller.service.RequireAuthentication(
		ctx, sessionID, authentication.Requirement{
			FreshWithin:     githubBindingRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	returnTo := safeReturnTo(req.ReturnTo)
	started, err := controller.module.Begin(ctx, identity.ID, sessionID, returnTo)
	if err != nil {
		return nil, mapGitHubBindingError(err)
	}
	return &v1.BeginGitHubBindingRes{
		AuthorizationURL: started.AuthorizationURL,
		ExpiresAt:        started.ExpiresAt.Format(time.RFC3339Nano),
	}, nil
}

func (controller *GitHubBindingController) ListGitHubBindings(
	ctx context.Context,
	_ *v1.ListGitHubBindingsReq,
) (*v1.ListGitHubBindingsRes, error) {
	if controller.module == nil {
		return nil, iderr.GitHubBindingUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	identity, err := controller.service.Me(
		ctx, request.Cookie.Get(sessionCookie, "").String(),
	)
	if err != nil {
		return nil, err
	}
	bindings, err := controller.module.List(ctx, identity.ID)
	if err != nil {
		return nil, mapGitHubBindingError(err)
	}
	result := make([]v1.GitHubBindingDTO, len(bindings))
	for index := range bindings {
		result[index] = githubBindingDTO(bindings[index])
	}
	return &v1.ListGitHubBindingsRes{Bindings: result}, nil
}

func (controller *GitHubBindingController) UnbindGitHubBinding(
	ctx context.Context,
	req *v1.UnbindGitHubBindingReq,
) (*v1.UnbindGitHubBindingRes, error) {
	if controller.module == nil {
		return nil, iderr.GitHubBindingUnavailable()
	}
	request := ghttp.RequestFromCtx(ctx)
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	_, identity, err := controller.service.RequireAuthentication(
		ctx, sessionID, authentication.Requirement{
			FreshWithin:     githubBindingRecentAuthentication,
			MinimumLevel:    authentication.LevelAAL1,
			RecoveryAllowed: false,
		},
	)
	if err != nil {
		return nil, err
	}
	binding, err := controller.module.Unbind(ctx, identity.ID, req.BindingID)
	if err != nil {
		return nil, mapGitHubBindingError(err)
	}
	ctx = actor.WithIdentity(ctx, identity.ID)
	controller.service.RecordGitHubBindingUnbound(ctx, binding)
	return &v1.UnbindGitHubBindingRes{Binding: githubBindingDTO(binding)}, nil
}

// GitHubCallback is a raw redirect endpoint. The server-side attempt binds state
// to the exact Identity session; the callback never trusts browser profile data.
func (controller *GitHubBindingController) GitHubCallback(request *ghttp.Request) {
	if controller.module == nil {
		request.Response.RedirectTo(withError(controller.fallbackURL, "github_binding_unavailable"))
		return
	}
	sessionID := request.Cookie.Get(sessionCookie, "").String()
	result, err := controller.module.Complete(
		request.Context(), request.Get("state").String(), sessionID,
		request.Get("code").String(),
	)
	if err != nil {
		controller.service.RecordGitHubBindingFailure(
			request.Context(), githubBindingRedirectError(err),
		)
		request.Response.RedirectTo(withError(
			controller.fallbackURL, githubBindingRedirectError(err),
		))
		return
	}
	ctx := actor.WithIdentity(request.Context(), result.Binding.IdentityID)
	controller.service.RecordGitHubBindingVerified(
		ctx, result.Binding, result.Created, result.Renamed,
	)
	request.Response.RedirectTo(result.ReturnTo)
}

// GitHubWebhook handles only the mandatory github_app_authorization/revoked
// event. Signature verification occurs over the unmodified request bytes.
func (controller *GitHubBindingController) GitHubWebhook(request *ghttp.Request) {
	if controller.module == nil || len(controller.webhookSecret) == 0 {
		request.Response.WriteStatus(http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Request.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		request.Response.WriteStatus(http.StatusBadRequest)
		return
	}
	if !githubbinding.VerifyWebhookSignature(
		controller.webhookSecret, body,
		request.Header.Get("X-Hub-Signature-256"),
	) {
		controller.service.RecordGitHubBindingFailure(
			request.Context(), "github_webhook_signature_invalid",
		)
		request.Response.WriteStatus(http.StatusUnauthorized)
		return
	}
	if request.Header.Get("X-GitHub-Event") != "github_app_authorization" {
		request.Response.WriteStatus(http.StatusNoContent)
		return
	}
	var payload struct {
		Action string `json:"action"`
		Sender struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		} `json:"sender"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Action != "revoked" ||
		payload.Sender.ID <= 0 {
		request.Response.WriteStatus(http.StatusBadRequest)
		return
	}
	bindings, err := controller.module.AuthorizationRevoked(
		request.Context(), strconv.FormatInt(payload.Sender.ID, 10),
		payload.Sender.Login,
	)
	if err != nil {
		request.Response.WriteStatus(http.StatusInternalServerError)
		return
	}
	for _, binding := range bindings {
		controller.service.RecordGitHubAuthorizationRevoked(
			request.Context(), binding,
			request.Header.Get("X-GitHub-Delivery"),
		)
	}
	request.Response.WriteStatus(http.StatusNoContent)
}

func githubBindingDTO(binding githubbinding.Binding) v1.GitHubBindingDTO {
	return v1.GitHubBindingDTO{
		ID: binding.ID, ProviderAccountID: binding.ProviderAccountID,
		ProviderNodeID: binding.ProviderNodeID, Login: binding.Login,
		AvatarURL: binding.AvatarURL, Status: binding.Status,
		VerifiedAt:     binding.VerifiedAt.Format(time.RFC3339Nano),
		LastVerifiedAt: binding.LastVerifiedAt.Format(time.RFC3339Nano),
		UnboundAt:      optionalGitHubBindingTime(binding.UnboundAt),
		BlockedAt:      optionalGitHubBindingTime(binding.BlockedAt),
	}
}

func optionalGitHubBindingTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func mapGitHubBindingError(err error) error {
	switch {
	case errors.Is(err, githubbinding.ErrInvalidAttempt):
		return iderr.GitHubBindingAttemptInvalid()
	case errors.Is(err, githubbinding.ErrBindingConflict):
		return iderr.GitHubBindingConflict()
	case errors.Is(err, githubbinding.ErrBindingNotFound),
		errors.Is(err, githubbinding.ErrBindingInactive):
		return iderr.GitHubBindingNotFound()
	case errors.Is(err, githubbinding.ErrProviderFailure):
		return iderr.GitHubProviderFailed()
	default:
		return iderr.GitHubBindingUnavailable()
	}
}

func githubBindingRedirectError(err error) string {
	switch {
	case errors.Is(err, githubbinding.ErrInvalidAttempt):
		return "github_binding_state"
	case errors.Is(err, githubbinding.ErrBindingConflict):
		return "github_binding_conflict"
	case errors.Is(err, githubbinding.ErrProviderFailure):
		return "github_binding_provider"
	default:
		return "github_binding_failed"
	}
}
