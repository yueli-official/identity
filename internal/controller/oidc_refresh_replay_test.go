package controller

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRefreshReplayInputBindsPublicAndConfidentialClients(t *testing.T) {
	public, _ := http.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt"}, "client_id": {"public-web"}}.Encode()))
	public.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if refresh, client := refreshReplayInput(public); refresh != "rt" || client != "public-web" {
		t.Fatalf("public = %q/%q", refresh, client)
	}
	confidential, _ := http.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"rt"}}.Encode()))
	confidential.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confidential.SetBasicAuth("service-web", "secret")
	if refresh, client := refreshReplayInput(confidential); refresh != "rt" || client != "service-web" {
		t.Fatalf("confidential = %q/%q", refresh, client)
	}
}
