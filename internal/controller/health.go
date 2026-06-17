// Package controller holds GoFrame HTTP handlers for identity-service.
package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
	"platform/gokit/response"
)

// Healthz is a liveness probe returning the platform OK envelope.
func Healthz(r *ghttp.Request) {
	r.Response.WriteJson(response.OK(map[string]any{"status": "up"}))
}
