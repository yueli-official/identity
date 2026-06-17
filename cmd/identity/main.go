// Command identity is the user-center IdP backend (milestone 2: identities,
// credentials, email/password login, self-hosted Redis session).
package main

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"

	"platform/services/identity/internal/controller"
)

func main() {
	ctx := gctx.New()
	s := g.Server()
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.GET("/healthz", controller.Healthz)
	})
	g.Log().Info(ctx, "identity-service starting")
	s.Run()
}
