package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/yueli-official/identity/internal/actor"
)

// ActorMiddleware reads request-scoped metadata (IP, User-Agent, X-Request-Id)
// from the incoming HTTP request and stores it as an [actor.Actor] in the
// request context so audit hooks in the logic layer can retrieve it via
// [actor.From].
func ActorMiddleware(r *ghttp.Request) {
	ctx := actor.With(r.Context(), actor.Actor{
		IP:        r.GetClientIp(),
		UserAgent: r.UserAgent(),
		RequestID: r.Header.Get("X-Request-Id"),
	})
	r.SetCtx(ctx)
	r.Middleware.Next()
}
