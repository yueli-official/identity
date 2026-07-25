package githubbinding

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGitHubAppUsesPKCEAndAuthenticatedUserEndpoint(t *testing.T) {
	var tokenForm url.Values
	var authorization string
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/token", func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		tokenForm, _ = url.ParseQuery(string(body))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"ghu_verified"}`))
	})
	mux.HandleFunc("/user", func(writer http.ResponseWriter, request *http.Request) {
		authorization = request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":1234567890123,"node_id":"U_1","login":"octocat","avatar_url":"https://images.test/a"}`))
	})
	mux.HandleFunc("/revoke", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})

	provider, err := newGitHubAppWithEndpoints(
		"Iv1.client", "secret", "https://account.test/callback",
		server.URL+"/authorize", server.URL+"/token", server.URL+"/user", server.URL+"/revoke",
	)
	if err != nil {
		t.Fatal(err)
	}
	authorizeURL, _ := url.Parse(provider.AuthorizationURL("state", "challenge"))
	if authorizeURL.Query().Get("code_challenge_method") != "S256" ||
		authorizeURL.Query().Get("code_challenge") != "challenge" {
		t.Fatalf("authorize URL = %s", authorizeURL)
	}
	token, err := provider.ExchangeCode(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if tokenForm.Get("code_verifier") != "verifier" {
		t.Fatalf("token form = %v", tokenForm)
	}
	account, err := provider.AuthenticatedUser(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if account.AccountID != "1234567890123" || account.Login != "octocat" ||
		authorization != "Bearer ghu_verified" {
		t.Fatalf("account=%+v authorization=%q", account, authorization)
	}
}

func TestVerifyWebhookSignatureRejectsTamperingAndSHA1(t *testing.T) {
	secret := []byte("webhook-secret")
	body := []byte(`{"action":"revoked"}`)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifyWebhookSignature(secret, body, signature) {
		t.Fatal("valid signature rejected")
	}
	if VerifyWebhookSignature(secret, append(body, ' '), signature) {
		t.Fatal("tampered body accepted")
	}
	if VerifyWebhookSignature(secret, body, strings.Replace(signature, "sha256=", "sha1=", 1)) {
		t.Fatal("SHA-1 header accepted")
	}
}
