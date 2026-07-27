package controller_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/google/uuid"

	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/oidc"
	"platform/services/identity/internal/repo"
	identityruntime "platform/services/identity/internal/runtime"
	"platform/services/identity/stepup"
)

func TestAdminMutationRequiresActionBoundOneTimeProof(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		store := repo.NewMemory()
		svc := logic.New(store, logic.DefaultConfig())
		manager, err := oidc.NewManager(ctx, store)
		t.AssertNil(err)
		proofVerifier, err := stepup.New(stepup.Config{
			Keys: manager, Issuer: "https://identity.example.test",
			Audience: "identity-api", Replay: stepup.NewMemoryReplayStore(),
		})
		t.AssertNil(err)
		ctl := controller.NewPrivacyAware(
			svc, nil, false, nil, time.Hour, proofVerifier,
		)

		server := g.Server(t.Name())
		server.SetAddr("127.0.0.1:0")
		server.SetDumpRouterMap(false)
		server.Use(identityruntime.APIMiddleware)
		server.Group("/", func(group *ghttp.RouterGroup) { group.Bind(ctl) })
		server.Start()
		defer server.Shutdown()
		base := fmt.Sprintf("http://127.0.0.1:%d", server.GetListenedPort())

		makeUser := func(email string) (string, string) {
			identity, createErr := svc.Register(ctx, logic.RegisterInput{
				Email: email, Password: "correct horse battery", DisplayName: email,
			})
			t.AssertNil(createErr)
			login, loginErr := svc.Login(ctx, logic.LoginInput{
				Email: email, Password: "correct horse battery",
			})
			t.AssertNil(loginErr)
			return identity.ID, "id_session=" + login.SessionID
		}
		adminID, adminCookie := makeUser("step-up-admin@example.test")
		t.AssertNil(svc.GrantRole(ctx, adminID, logic.AdminRole))
		targetID, _ := makeUser("step-up-target@example.test")
		path := "/api/v1/admin/identities/" + targetID + "/roles?role=admin"

		request := func(proof string) (int, string) {
			req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, base+path, nil)
			t.AssertNil(requestErr)
			req.Header.Set("Cookie", adminCookie)
			if proof != "" {
				req.Header.Set("X-Step-Up-Proof", proof)
			}
			response, responseErr := http.DefaultClient.Do(req)
			t.AssertNil(responseErr)
			defer response.Body.Close()
			body := new(strings.Builder)
			_, _ = io.Copy(body, response.Body)
			return response.StatusCode, body.String()
		}
		status, body := request("")
		t.Assert(status, http.StatusPreconditionRequired)
		t.Assert(gjson.New(body).Get("code").String(), "identity.step_up_required")

		mint := func(resource string) string {
			digest := sha256.Sum256([]byte(resource))
			now := time.Now().UTC()
			raw, mintErr := manager.MintStepUpProof(oidc.StepUpProofInput{
				Issuer: "https://identity.example.test", ID: uuid.NewString(),
				Subject: adminID, SessionID: "session-proof",
				Audience: "identity-api", Action: "identity.admin.role.grant",
				ResourceDigest: digest[:],
				Authentication: authentication.MultiFactor(
					authentication.Password(uuid.NewString(), now),
					uuid.NewString(), now, "totp-1",
				),
				IssuedAt: now, TTL: 2 * time.Minute,
			})
			t.AssertNil(mintErr)
			return raw
		}
		status, body = request(mint("identity:wrong:role:admin"))
		t.Assert(status, http.StatusUnauthorized)
		t.Assert(gjson.New(body).Get("code").String(), "identity.step_up_proof_invalid")

		proof := mint("identity:" + targetID + ":role:admin")
		status, _ = request(proof)
		t.Assert(status, http.StatusOK)
		status, body = request(proof)
		t.Assert(status, http.StatusConflict)
		t.Assert(gjson.New(body).Get("code").String(), "identity.step_up_proof_replayed")
	})
}
