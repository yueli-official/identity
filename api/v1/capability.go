package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"platform/gokit/capability"
)

type AdminCapabilitiesReq struct {
	g.Meta `path:"/api/v1/admin/capabilities" method:"get" tags:"identity-admin" summary:"List identity capabilities" security:"AdminAuth"`
}
type AdminCapabilitiesRes struct {
	Manifest capability.Manifest `json:"manifest"`
}

type AdminCapabilityReq struct {
	g.Meta `path:"/api/v1/admin/capabilities/{key}" method:"get" tags:"identity-admin" summary:"Get an identity capability" security:"AdminAuth"`
	Key    string `json:"key" in:"path" v:"required"`
}
type AdminCapabilityRes struct {
	Capability capability.Capability `json:"capability"`
}

type AdminProvidersReq struct {
	g.Meta `path:"/api/v1/admin/providers" method:"get" tags:"identity-admin" summary:"List identity providers" security:"AdminAuth"`
}
type AdminProvidersRes struct {
	Items []capability.Provider `json:"items"`
}

type AdminProviderReq struct {
	g.Meta `path:"/api/v1/admin/providers/{key}" method:"get" tags:"identity-admin" summary:"Get an identity provider" security:"AdminAuth"`
	Key    string `json:"key" in:"path" v:"required"`
}
type AdminProviderRes struct {
	Provider capability.Provider `json:"provider"`
}

type AdminProviderHealthCheckReq struct {
	g.Meta `path:"/api/v1/admin/providers/{key}/health-check" method:"post" tags:"identity-admin" summary:"Probe an identity provider without a business action" security:"AdminAuth"`
	Key    string `json:"key" in:"path" v:"required"`
}
type AdminProviderHealthCheckRes struct {
	Provider capability.Provider `json:"provider"`
}
