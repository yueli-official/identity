package controller

import (
	"testing"
)

func TestDiscoveryAdvertisesRefreshAndLogout(t *testing.T) {
	c := &OIDCController{issuer: "http://localhost:8081"}
	doc := c.discoveryDoc()
	scopes, _ := doc["scopes_supported"].([]string)
	if !contains(scopes, "offline_access") {
		t.Errorf("scopes_supported missing offline_access: %v", scopes)
	}
	grants, _ := doc["grant_types_supported"].([]string)
	if !contains(grants, "refresh_token") {
		t.Errorf("grant_types_supported missing refresh_token: %v", grants)
	}
	if doc["revocation_endpoint"] == nil || doc["end_session_endpoint"] == nil {
		t.Errorf("missing revocation/end_session endpoint")
	}
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
