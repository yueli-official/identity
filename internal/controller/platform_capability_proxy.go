package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"platform/gokit/response"
	"platform/services/identity/internal/oidc"
)

type CapabilityProxyTarget struct {
	BaseURL  string
	Audience string
}

// PlatformCapabilityProxy is the narrow session-to-service-identity bridge
// used by the Account BFF. It only forwards capability discovery operations.
type PlatformCapabilityProxy struct {
	base    *Controller
	mgr     *oidc.Manager
	issuer  string
	targets map[string]CapabilityProxyTarget
}

var capabilityProxyIdentifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func NewPlatformCapabilityProxy(base *Controller, mgr *oidc.Manager, issuer string, targets map[string]CapabilityProxyTarget) (*PlatformCapabilityProxy, error) {
	copyTargets := make(map[string]CapabilityProxyTarget, len(targets))
	for key, target := range targets {
		key = strings.TrimSpace(key)
		target.BaseURL = strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
		target.Audience = strings.TrimSpace(target.Audience)
		if !capabilityProxyIdentifier.MatchString(key) {
			return nil, fmt.Errorf("invalid capability proxy target key %q", key)
		}
		if err := validateCapabilityProxyOrigin(target.BaseURL); err != nil {
			return nil, fmt.Errorf("invalid capability proxy target %q: %w", key, err)
		}
		if target.Audience == "" {
			return nil, fmt.Errorf("capability proxy target %q audience is required", key)
		}
		copyTargets[key] = target
	}
	return &PlatformCapabilityProxy{base: base, mgr: mgr, issuer: issuer, targets: copyTargets}, nil
}

func (proxy *PlatformCapabilityProxy) Forward(request *ghttp.Request) {
	adminID, err := proxy.base.requireAdmin(request.Context())
	if err != nil {
		request.SetError(err)
		return
	}
	service, suffix, scope, ok := capabilityProxyRoute(request.Method, request.Request.URL.Path)
	if !ok {
		request.Response.WriteStatus(404)
		return
	}
	target, ok := proxy.targets[service]
	if !ok {
		request.Response.WriteStatus(404)
		return
	}
	bearer, err := proxy.mgr.MintServiceToken(proxy.issuer, adminID, target.Audience, scope, 2*time.Minute, time.Now())
	if err != nil {
		request.SetError(err)
		return
	}
	upstreamURL := target.BaseURL + "/api/v1/admin/" + escapeCapabilityProxySuffix(suffix)
	upstreamCtx, cancel := context.WithTimeout(request.Context(), 9*time.Second)
	defer cancel()
	upstreamRequest, err := http.NewRequestWithContext(upstreamCtx, request.Method, upstreamURL, nil)
	if err != nil {
		request.SetError(err)
		return
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+bearer)
	upstreamRequest.Header.Set("Accept", "application/json")
	origin, _ := url.Parse(target.BaseURL)
	client := &http.Client{CheckRedirect: sameOriginRedirectPolicy(origin)}
	upstream, err := client.Do(upstreamRequest)
	if err != nil {
		request.SetError(fmt.Errorf("%s capability service unreachable: %w", service, err))
		return
	}
	defer upstream.Body.Close()
	body, err := io.ReadAll(io.LimitReader(upstream.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		request.SetError(fmt.Errorf("%s capability response is invalid or too large", service))
		return
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		request.SetError(fmt.Errorf("%s capability response is not valid JSON", service))
		return
	}
	if payload.Code != "ok" {
		request.Response.WriteStatus(upstream.StatusCode)
		request.Response.WriteJson(response.Fail(payload.Code, payload.Message, nil))
		return
	}
	request.Response.WriteJson(response.OK(payload.Data))
}

func capabilityProxyRoute(method, path string) (service, suffix, scope string, ok bool) {
	prefix := "/api/v1/admin/platform-proxy/"
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if !strings.HasPrefix(path, prefix) || len(parts) < 2 {
		return "", "", "", false
	}
	service = parts[0]
	suffix = strings.Join(parts[1:], "/")
	if !capabilityProxyIdentifier.MatchString(service) {
		return "", "", "", false
	}
	switch {
	case method == "GET" && (suffix == "capabilities" || suffix == "providers"):
		return service, suffix, "platform:capabilities:read", true
	case method == "GET" && len(parts) == 3 && (parts[1] == "capabilities" || parts[1] == "providers") && capabilityProxyIdentifier.MatchString(parts[2]):
		return service, suffix, "platform:capabilities:read", true
	case method == "POST" && len(parts) == 4 && parts[1] == "providers" && capabilityProxyIdentifier.MatchString(parts[2]) && parts[3] == "health-check":
		return service, suffix, "platform:capabilities:probe", true
	default:
		return "", "", "", false
	}
}

func validateCapabilityProxyOrigin(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("an http or https origin is required")
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("userinfo, path, query, and fragment are forbidden")
	}
	return nil
}

func escapeCapabilityProxySuffix(suffix string) string {
	parts := strings.Split(suffix, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func sameOriginRedirectPolicy(origin *url.URL) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many capability proxy redirects")
		}
		if !strings.EqualFold(request.URL.Scheme, origin.Scheme) || !strings.EqualFold(request.URL.Host, origin.Host) {
			return fmt.Errorf("capability proxy redirect changed origin")
		}
		return nil
	}
}
