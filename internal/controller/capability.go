package controller

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"

	foundationauth "github.com/yueli-official/foundation/go/auth"
	"github.com/yueli-official/foundation/go/capability"
	"github.com/yueli-official/foundation/go/goframe/ratelimit"
	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/actor"
	"platform/services/identity/internal/identitycap"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

type Capability struct {
	auth          *Controller
	registry      *identitycap.Registry
	audit         repo.AuditRepo
	service       capability.ServiceMetadata
	healthLimiter *ratelimit.Limiter
}

type capabilityActor struct {
	rateKey    string
	identityID string
	clientID   string
}

func NewCapability(auth *Controller, registry *identitycap.Registry, audit repo.AuditRepo, service capability.ServiceMetadata) *Capability {
	return &Capability{
		auth: auth, registry: registry, audit: audit, service: service,
		healthLimiter: ratelimit.MustNew(ratelimit.Policy{Limit: 5, Window: time.Minute}),
	}
}

func (controller *Capability) Capabilities(ctx context.Context, _ *v1.AdminCapabilitiesReq) (*v1.AdminCapabilitiesRes, error) {
	snapshot, err := controller.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminCapabilitiesRes{Manifest: snapshot.Manifest()}, nil
}

func (controller *Capability) Capability(ctx context.Context, req *v1.AdminCapabilityReq) (*v1.AdminCapabilityRes, error) {
	snapshot, err := controller.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	item, ok := snapshot.Capability(req.Key)
	if !ok {
		return nil, iderr.CapabilityNotFound(req.Key)
	}
	return &v1.AdminCapabilityRes{Capability: item}, nil
}

func (controller *Capability) Providers(ctx context.Context, _ *v1.AdminProvidersReq) (*v1.AdminProvidersRes, error) {
	snapshot, err := controller.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminProvidersRes{Items: snapshot.ListProviders()}, nil
}

func (controller *Capability) Provider(ctx context.Context, req *v1.AdminProviderReq) (*v1.AdminProviderRes, error) {
	snapshot, err := controller.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	item, ok := snapshot.Provider(req.Key)
	if !ok {
		return nil, iderr.ProviderNotFound(req.Key)
	}
	return &v1.AdminProviderRes{Provider: item}, nil
}

func (controller *Capability) ProviderHealthCheck(ctx context.Context, req *v1.AdminProviderHealthCheckReq) (*v1.AdminProviderHealthCheckRes, error) {
	probeActor, err := controller.authorize(ctx, "platform:capabilities:probe")
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.Key)
	if decision := controller.healthLimiter.Evaluate(probeActor.rateKey + "|" + key); !decision.Allowed {
		return nil, iderr.CapabilityProbeRateLimited(key)
	}
	snapshot, err := controller.registry.Snapshot(controller.service, time.Now())
	if err != nil {
		return nil, err
	}
	if _, ok := snapshot.Provider(key); !ok {
		return nil, iderr.ProviderNotFound(key)
	}
	if controller.audit == nil {
		return nil, iderr.CapabilityAuditUnavailable()
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	startedAt := time.Now()
	probeErr := controller.registry.CheckHealth(probeCtx, key)
	cancel()
	result := "success"
	if probeErr != nil {
		result = "failure"
	}
	auditCtx, auditCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer auditCancel()
	requestActor := actor.From(ctx)
	if err := controller.audit.InsertAudit(auditCtx, repo.AuditRow{
		Event: "capability.provider_health_check", ActorID: probeActor.identityID, ClientID: probeActor.clientID, Result: result,
		IP: requestActor.IP, UserAgent: requestActor.UserAgent, RequestID: requestActor.RequestID,
		Detail: map[string]any{"provider": key, "result": result, "durationMs": time.Since(startedAt).Milliseconds()},
	}); err != nil {
		return nil, iderr.CapabilityAuditUnavailable()
	}
	snapshot, snapshotErr := controller.registry.Snapshot(controller.service, time.Now())
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	item, _ := snapshot.Provider(key)
	g.Log().Info(ctx, "identity provider health check", "provider", key, "health", item.Health, "actor", probeActor.rateKey, "durationMs", time.Since(startedAt).Milliseconds(), "succeeded", probeErr == nil)
	return &v1.AdminProviderHealthCheckRes{Provider: item}, nil
}

func (controller *Capability) snapshot(ctx context.Context) (*capability.Snapshot, error) {
	if _, err := controller.authorize(ctx, "platform:capabilities:read"); err != nil {
		return nil, err
	}
	return controller.registry.Snapshot(controller.service, time.Now())
}

func (controller *Capability) authorize(ctx context.Context, scope string) (capabilityActor, error) {
	if principal, ok := foundationauth.FromContext(ctx); ok && principal != nil {
		if principal.HasRole(logic.AdminRole) || principal.HasScope(scope) {
			result := capabilityActor{rateKey: principal.ActorKey(), clientID: principal.ClientID}
			if uuid.Validate(principal.Subject) == nil {
				result.identityID = principal.Subject
			}
			return result, nil
		}
		return capabilityActor{}, iderr.Forbidden()
	}
	identityID, err := controller.auth.requireAdmin(ctx)
	return capabilityActor{rateKey: identityID, identityID: identityID}, err
}
