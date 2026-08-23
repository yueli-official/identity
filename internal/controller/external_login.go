package controller

import (
	"context"
	"errors"
	"time"

	v1 "github.com/yueli-official/identity/api/v1"
	"github.com/yueli-official/identity/internal/externallogin"
	"github.com/yueli-official/identity/internal/iderr"
)

type ExternalLoginController struct {
	base    *Controller
	manager *externallogin.Manager
}

func NewExternalLoginController(base *Controller, manager *externallogin.Manager) *ExternalLoginController {
	return &ExternalLoginController{base: base, manager: manager}
}

func (controller *ExternalLoginController) PublicProviders(ctx context.Context, _ *v1.PublicExternalLoginProvidersReq) (*v1.PublicExternalLoginProvidersRes, error) {
	views, err := controller.manager.Public(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]v1.PublicExternalLoginProvider, 0, len(views))
	for _, view := range views {
		entries = append(entries, v1.PublicExternalLoginProvider{
			Key: view.Key, Label: view.Label, RegistrationPolicy: string(view.RegistrationPolicy),
		})
	}
	return &v1.PublicExternalLoginProvidersRes{Entries: entries}, nil
}

func (controller *ExternalLoginController) AdminProviders(ctx context.Context, _ *v1.AdminExternalLoginProvidersReq) (*v1.AdminExternalLoginProvidersRes, error) {
	if _, err := controller.base.requireAdmin(ctx); err != nil {
		return nil, err
	}
	views, err := controller.manager.List(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]v1.ExternalLoginProviderView, 0, len(views))
	for _, view := range views {
		entries = append(entries, externalLoginView(view))
	}
	return &v1.AdminExternalLoginProvidersRes{Entries: entries}, nil
}

func (controller *ExternalLoginController) AdminSaveProvider(ctx context.Context, request *v1.AdminSaveExternalLoginProviderReq) (*v1.AdminSaveExternalLoginProviderRes, error) {
	adminID, err := controller.base.requireAdminAction(
		ctx, "identity.external_login_provider.save", "identity:external-login:"+request.Key,
	)
	if err != nil {
		return nil, err
	}
	view, err := controller.manager.Save(ctx, externallogin.SaveInput{
		ActorID: adminID,
		Key:     request.Key, ClientID: request.ClientID,
		ClientSecret: request.ClientSecret, Enabled: request.Enabled,
	})
	if err != nil {
		if errors.Is(err, externallogin.ErrInvalid) {
			return nil, iderr.ExternalLoginProviderInvalid(request.Key)
		}
		return nil, err
	}
	return &v1.AdminSaveExternalLoginProviderRes{Provider: externalLoginView(view)}, nil
}

func (controller *ExternalLoginController) AdminCheckProvider(ctx context.Context, request *v1.AdminCheckExternalLoginProviderReq) (*v1.AdminCheckExternalLoginProviderRes, error) {
	adminID, err := controller.base.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	err = controller.manager.CheckHealth(ctx, request.Key, adminID)
	if err != nil {
		return &v1.AdminCheckExternalLoginProviderRes{Healthy: false, Message: err.Error()}, nil
	}
	return &v1.AdminCheckExternalLoginProviderRes{Healthy: true}, nil
}

func externalLoginView(view externallogin.View) v1.ExternalLoginProviderView {
	result := v1.ExternalLoginProviderView{
		Key: view.Key, Label: view.Label,
		RegistrationPolicy: string(view.RegistrationPolicy), Configured: view.Configured,
		Enabled: view.Enabled, ClientID: view.ClientID, RedirectURL: view.RedirectURL,
		SecretVersion: view.SecretVersion, LastHealthOK: view.LastHealthOK,
		LastHealthError: view.LastHealthError,
	}
	if view.LastHealthChecked != nil {
		result.LastHealthChecked = view.LastHealthChecked.Format(time.RFC3339)
	}
	if view.UpdatedAt != nil {
		result.UpdatedAt = view.UpdatedAt.Format(time.RFC3339)
	}
	return result
}
