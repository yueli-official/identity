// Command identity is the user-center IdP backend (milestone 2: identities,
// credentials, email/password login, self-hosted Redis session).
package main

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"

	"platform/gokit/ghttpx"
	"platform/services/identity/internal/cache"
	"platform/services/identity/internal/controller"
	"platform/services/identity/internal/dao"
	"platform/services/identity/internal/logic"
	"platform/services/identity/internal/repo"
)

func main() {
	ctx := gctx.New()

	db := g.DB()     // configured via manifest/config + GF_DATABASE_DEFAULT_LINK env
	rdb := g.Redis() // configured via manifest/config + GF_REDIS_DEFAULT_ADDRESS env

	store := repo.NewComposite(dao.NewPG(db), cache.NewRedis(rdb), cache.NewRedis(rdb))
	svc := logic.New(store, logic.DefaultConfig())

	secureCookie := g.Cfg().MustGet(ctx, "cookie.secure", true).Bool()
	ctl := controller.New(svc, secureCookie)

	s := g.Server()
	s.Use(ghttpx.Middleware)
	s.Group("/", func(grp *ghttp.RouterGroup) {
		grp.GET("/healthz", controller.Healthz)
		grp.Bind(ctl)
	})
	g.Log().Info(ctx, "identity-service starting")
	s.Run()
}
