package controller_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/publisher"
	"platform/services/identity/internal/repo"
)

func TestPublisherAttestationHTTPJourney(t *testing.T) {
	store := repo.NewMemory()
	service := logic.New(store, logic.DefaultConfig())
	keys, err := publisher.NewLocalKeyProvider()
	if err != nil {
		t.Fatal(err)
	}
	module, err := publisher.New(publisher.Config{
		Issuer: "https://identity.publisher-http.test",
		Consumers: []publisher.Consumer{{
			Audience: "urn:yueli:registry:yotta", Instance: "urn:yueli:platform-instance:test",
			ArtifactKinds: map[string]publisher.ArtifactPolicy{
				"workflow-release": {MediaType: "application/vnd.yueli.workflow-release.v1+json"},
			},
		}},
		Store: publisher.NewMemoryStore(), Signer: keys,
	})
	if err != nil {
		t.Fatal(err)
	}

	server := g.Server(t.Name())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttpx.Middleware)
		group.Bind(controller.New(service, false))
		group.Bind(controller.NewPublisher(service, module, keys))
	})
	server.Start()
	defer server.Shutdown()
	base := fmt.Sprintf("http://127.0.0.1:%d", server.GetListenedPort())

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	command := map[string]any{
		"idempotencyKey":   "publisher-http-request-0001",
		"audience":         "urn:yueli:registry:yotta",
		"consumerInstance": "urn:yueli:platform-instance:test",
		"namespace":        "example",
		"artifact": map[string]any{
			"kind": "workflow-release", "identity": "example/tool", "version": "1.0.0",
			"name": "workflow:example/tool@1.0.0", "uri": "pkg:yueli-workflow/example/tool@1.0.0",
			"mediaType": "application/vnd.yueli.workflow-release.v1+json",
			"sha256":    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	unauthenticated := postJSON(t, client, base+"/api/v1/account/publisher-attestations", command)
	if code := gjson.New(unauthenticated).Get("code").String(); code != "identity.not_authenticated" {
		t.Fatalf("unauthenticated code = %q, body=%s", code, unauthenticated)
	}

	postJSON(t, client, base+"/api/v1/auth/register", map[string]any{
		"email": "publisher@example.test", "password": "Jade river orbits nightly 58!",
		"displayName": "Publisher",
	})
	postJSON(t, client, base+"/api/v1/auth/login", map[string]any{
		"email": "publisher@example.test", "password": "Jade river orbits nightly 58!",
	})

	firstBody := postJSON(t, client, base+"/api/v1/account/publisher-attestations", command)
	first := gjson.New(firstBody)
	if first.Get("attestationId").String() == "" ||
		first.Get("statementDigest").String() == "" ||
		first.Get("envelope").String() == "" {
		t.Fatalf("issue response = %s", firstBody)
	}
	secondBody := postJSON(t, client, base+"/api/v1/account/publisher-attestations", command)
	if first.Get("envelope").String() != gjson.New(secondBody).Get("envelope").String() {
		t.Fatalf("idempotent HTTP issue returned different envelope")
	}

	keySet := getEnvelope(t, client, base+"/api/v1/publisher/verification-keys")
	if keySet.Get("keys.0.purpose").String() != publisher.KeyPurpose ||
		keySet.Get("keys.0.publicJwk.d").String() != "" {
		t.Fatalf("verification keys = %s", keySet.MustToJsonString())
	}
}
