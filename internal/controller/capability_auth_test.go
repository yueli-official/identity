package controller

import (
	"context"
	"errors"
	"testing"

	"platform/gokit/authjwt"
	"platform/gokit/capability"
	"platform/gokit/errs"
	v1 "platform/services/identity/api/v1"
	"platform/services/identity/internal/identitycap"
	"platform/services/identity/internal/iderr"
	"platform/services/identity/internal/repo"
)

type captureCapabilityAudit struct{ row repo.AuditRow }

func (audit *captureCapabilityAudit) InsertAudit(_ context.Context, row repo.AuditRow) error {
	audit.row = row
	return nil
}
func (*captureCapabilityAudit) QueryAudit(context.Context, repo.AuditFilter) ([]repo.AuditRow, error) {
	return nil, nil
}

func TestCapabilityMachineAuthorizationSeparatesReadAndProbeScopes(t *testing.T) {
	controller := &Capability{}
	readCtx := authjwt.WithPrincipal(context.Background(), &authjwt.Principal{ClientID: "account-platform", Scopes: []string{"platform:capabilities:read"}})
	actor, err := controller.authorize(readCtx, "platform:capabilities:read")
	if err != nil || actor.rateKey != "account-platform" || actor.clientID != "account-platform" || actor.identityID != "" {
		t.Fatalf("read authorization actor=%+v err=%v", actor, err)
	}
	if _, err := controller.authorize(readCtx, "platform:capabilities:probe"); err == nil {
		t.Fatal("read token must not probe")
	} else {
		var coded *errs.Coded
		if !errors.As(err, &coded) || coded.Code != iderr.CodeForbidden {
			t.Fatalf("read token probe error = %v", err)
		}
	}
	probeCtx := authjwt.WithPrincipal(context.Background(), &authjwt.Principal{Subject: "operator", Scopes: []string{"platform:capabilities:probe"}})
	actor, err = controller.authorize(probeCtx, "platform:capabilities:probe")
	if err != nil || actor.rateKey != "operator" || actor.identityID != "" {
		t.Fatalf("probe authorization actor=%+v err=%v", actor, err)
	}
	userCtx := authjwt.WithPrincipal(context.Background(), &authjwt.Principal{Subject: "4f553f75-e2d9-4f21-8d12-5f43659504f2", ClientID: "account-platform", Scopes: []string{"platform:capabilities:probe"}})
	actor, err = controller.authorize(userCtx, "platform:capabilities:probe")
	if err != nil || actor.identityID != "4f553f75-e2d9-4f21-8d12-5f43659504f2" || actor.clientID != "account-platform" {
		t.Fatalf("user-scoped machine actor=%+v err=%v", actor, err)
	}
}

func TestMachineProbeAuditsClientIDWithoutInvalidUUIDActor(t *testing.T) {
	registry, err := identitycap.New(identitycap.Registration{
		Key: "dev-mail", Adapter: "dev", Registered: true, Enabled: true,
		CapabilityKeys: []string{"identity.verify-email"}, Operations: []string{"send"},
		Checker: identitycap.HealthCheckFunc(func(context.Context) error { return nil }),
	})
	if err != nil {
		t.Fatal(err)
	}
	audit := &captureCapabilityAudit{}
	controller := NewCapability(nil, registry, audit, capability.ServiceMetadata{Name: "identity", Version: "test", BuildSHA: "test", Deployment: "identity-test"})
	ctx := authjwt.WithPrincipal(context.Background(), &authjwt.Principal{Subject: "account-platform", ClientID: "account-platform", Scopes: []string{"platform:capabilities:probe"}})
	if _, err := controller.ProviderHealthCheck(ctx, &v1.AdminProviderHealthCheckReq{Key: "dev-mail"}); err != nil {
		t.Fatal(err)
	}
	if audit.row.ActorID != "" || audit.row.ClientID != "account-platform" {
		t.Fatalf("machine audit row = %+v", audit.row)
	}
}
