package runtime

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	goframeapi "github.com/yueli-official/foundation/go/goframe/api"
	"github.com/yueli-official/foundation/go/goframe/ratelimit"
	"github.com/yueli-official/identity/internal/iderr"
)

const defaultRateLimitPerMinute = 600

var unmeteredAPIMiddleware = mustAPIMiddleware(nil)

// APIMiddleware is the unmetered adapter used by focused controller tests.
// Production entry points should explicitly provide their process limiter.
func APIMiddleware(request *ghttp.Request) {
	unmeteredAPIMiddleware.Handle(request)
}

func MustAPIMiddleware(limiter *ratelimit.Limiter) *goframeapi.Middleware {
	return mustAPIMiddleware(limiter)
}

func mustAPIMiddleware(limiter *ratelimit.Limiter) *goframeapi.Middleware {
	options := goframeapi.Options{
		TraceHeader: traceHeader,
		RateLimited: iderr.DescriptorRateLimited,
		Validation:  iderr.DescriptorValidation,
		Internal:    iderr.DescriptorInternal,
	}
	if limiter != nil {
		options.Limiter = limiter
		options.ClientKey = goframeapi.ForwardedClientIPKey
	}
	middleware, err := goframeapi.New(options)
	if err != nil {
		panic(err)
	}
	return middleware
}

func MustRateLimiterFromEnvironment() *ratelimit.Limiter {
	value, err := rateLimiterFromEnvironment()
	if err != nil {
		panic(err)
	}
	return value
}

func rateLimiterFromEnvironment() (*ratelimit.Limiter, error) {
	limit := defaultRateLimitPerMinute
	if raw := strings.TrimSpace(os.Getenv("PLATFORM_RATE_LIMIT_PER_MINUTE")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return nil, fmt.Errorf("PLATFORM_RATE_LIMIT_PER_MINUTE must be a non-negative integer")
		}
		limit = parsed
	}
	return ratelimit.New(ratelimit.Policy{Limit: limit, Window: time.Minute})
}

// NewRawRateLimitMiddleware protects OIDC and OAuth handlers that own an RFC
// response contract instead of the Foundation Problem body.
func NewRawRateLimitMiddleware(limiter *ratelimit.Limiter, clientKey goframeapi.ClientKey) func(*ghttp.Request) {
	if limiter == nil || clientKey == nil {
		panic("identity runtime raw rate-limit middleware requires limiter and client key policy")
	}
	return func(request *ghttp.Request) {
		traceID := ensureTraceID(request)
		decision := limiter.Evaluate(clientKey(request))
		for key, value := range decision.Headers() {
			request.Response.Header().Set(key, value)
		}
		if decision.Allowed {
			request.Middleware.Next()
			return
		}
		request.Response.WriteHeader(http.StatusTooManyRequests)
		request.Response.WriteJson(map[string]string{
			"error":             "temporarily_unavailable",
			"error_description": "rate limit exceeded",
			"trace_id":          traceID,
		})
	}
}

func ensureTraceID(request *ghttp.Request) string {
	traceID := request.Header.Get(traceHeader)
	if traceID == "" {
		traceID = guid.S()
	}
	request.Response.Header().Set(traceHeader, traceID)
	return traceID
}
