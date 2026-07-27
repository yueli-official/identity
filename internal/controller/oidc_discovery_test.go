package controller

import (
	"net/url"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/authentication"
	"github.com/yueli-official/identity/internal/model"
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
	acr, _ := doc["acr_values_supported"].([]string)
	if !contains(acr, string(authentication.ProfilePhishingResistant)) {
		t.Errorf("acr_values_supported missing phishing-resistant profile: %v", acr)
	}
	claims, _ := doc["claims_supported"].([]string)
	for _, claim := range []string{"auth_time", "acr", "amr"} {
		if !contains(claims, claim) {
			t.Errorf("claims_supported missing %s: %v", claim, claims)
		}
	}
}

func TestActiveAuthenticationRequiredUsesAuthenticationEventTime(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	session := model.Session{Authentication: authentication.Password("event-1", now.Add(-time.Minute))}

	needed, err := activeAuthenticationRequired(url.Values{"max_age": {"120"}}, session, now)
	if err != nil || needed {
		t.Fatalf("fresh decision = %t, %v", needed, err)
	}
	needed, err = activeAuthenticationRequired(url.Values{"max_age": {"30"}}, session, now)
	if err != nil || !needed {
		t.Fatalf("stale decision = %t, %v", needed, err)
	}
	needed, err = activeAuthenticationRequired(url.Values{"max_age": {"0"}}, session, now)
	if err != nil || !needed {
		t.Fatalf("max_age=0 decision = %t, %v", needed, err)
	}
}

func TestActiveAuthenticationRequiredHonorsPromptLoginAndRejectsBadMaxAge(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	session := model.Session{Authentication: authentication.Password("event-1", now)}

	needed, err := activeAuthenticationRequired(url.Values{"prompt": {"consent login"}}, session, now)
	if err != nil || !needed {
		t.Fatalf("prompt decision = %t, %v", needed, err)
	}
	if _, err := activeAuthenticationRequired(url.Values{"max_age": {"-1"}}, session, now); err == nil {
		t.Fatal("negative max_age accepted")
	}
}

func TestOIDCReauthenticationMarkerURLRoundTrip(t *testing.T) {
	original, err := removeQueryValue("/oauth2/authorize?state=s&client_id=c", oidcReauthParam)
	if err != nil {
		t.Fatal(err)
	}
	withMarker, err := addQueryValue(original, oidcReauthParam, "nonce")
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := removeQueryValue(withMarker, oidcReauthParam)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip != original {
		t.Fatalf("round trip = %q, want %q", roundTrip, original)
	}
}

func TestAuthenticationContextAcceptedRequiresOneRequestedACR(t *testing.T) {
	session := model.Session{
		Authentication: authentication.Password("event-1", time.Now().UTC()),
	}
	if !authenticationContextAccepted(url.Values{}, session) {
		t.Fatal("request without acr_values rejected")
	}
	if !authenticationContextAccepted(url.Values{
		"acr_values": {"urn:other " + string(authentication.ProfileBaseline)},
	}, session) {
		t.Fatal("matching acr rejected")
	}
	if authenticationContextAccepted(url.Values{
		"acr_values": {string(authentication.ProfilePhishingResistant)},
	}, session) {
		t.Fatal("lower assurance silently accepted")
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
