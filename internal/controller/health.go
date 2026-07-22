// Package controller holds GoFrame HTTP handlers for identity-service.
package controller

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// Healthz is a liveness probe returning the raw health DTO.
func Healthz(r *ghttp.Request) {
	r.Response.WriteJson(map[string]any{"status": "up"})
}
