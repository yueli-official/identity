package controller_test

import (
	"fmt"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"

	"platform/services/identity/internal/controller"
)

// TestHealthz verifies that GET /healthz returns a JSON envelope with code == "ok".
func TestHealthz(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		s := g.Server(t.Name())
		s.Group("/", func(group *ghttp.RouterGroup) {
			group.GET("/healthz", controller.Healthz)
		})
		s.SetDumpRouterMap(false)
		s.Start()
		defer s.Shutdown()

		port := s.GetListenedPort()
		client := g.Client()
		client.SetPrefix(fmt.Sprintf("http://127.0.0.1:%d", port))

		resp := client.GetContent(nil, "/healthz")
		t.Assert(g.NewVar(resp).MapStrAny()["code"], "ok")
	})
}
