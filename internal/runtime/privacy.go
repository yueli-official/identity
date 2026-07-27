package runtime

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	foundationauth "github.com/yueli-official/foundation/go/auth"
	"github.com/yueli-official/foundation/go/privacy"
	privacygoframe "github.com/yueli-official/foundation/go/privacy/goframe"
	"platform/services/identity/internal/iderr"
)

func PrivacyOwnerHandler(host privacy.OwnerHost, requiredScope string) ghttp.HandlerFunc {
	handler, err := privacygoframe.NewOwnerHandler(host)
	if err != nil {
		panic(err)
	}
	scope := strings.TrimSpace(requiredScope)
	if scope == "" {
		scope = "privacy:owner"
	}
	return func(request *ghttp.Request) {
		principal, ok := foundationauth.FromContext(request.Context())
		if !ok || principal == nil || !principal.HasScope(scope) {
			request.SetError(iderr.Forbidden())
			return
		}
		handler(request)
	}
}
