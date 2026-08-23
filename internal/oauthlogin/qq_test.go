package oauthlogin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestQQAuthorizeExchangeAndFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("code") != "good-code" || request.URL.Query().Get("client_secret") != "secret" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"QQ-AT"}`))
	})
	mux.HandleFunc("/me", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("access_token") != "QQ-AT" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = writer.Write([]byte(`callback( {"client_id":"appid","openid":"openid-1","unionid":"union-1"} );`))
	})
	mux.HandleFunc("/userinfo", func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("openid") != "openid-1" || query.Get("oauth_consumer_key") != "appid" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ret":0,"nickname":"月离用户","figureurl_qq_2":"https://q.example/avatar"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	provider := newQQWithEndpoints("appid", "secret", "https://account.example/callback", server.URL+"/auth", server.URL+"/token", server.URL+"/me", server.URL+"/userinfo")
	authorize, err := url.Parse(provider.AuthorizeURL("STATE"))
	if err != nil {
		t.Fatal(err)
	}
	if authorize.Query().Get("client_id") != "appid" || authorize.Query().Get("scope") != "get_user_info" || authorize.Query().Get("state") != "STATE" {
		t.Fatalf("authorize URL = %s", authorize)
	}
	token, err := provider.ExchangeCode(context.Background(), "good-code")
	if err != nil || token != "QQ-AT" {
		t.Fatalf("exchange = %q, %v", token, err)
	}
	profile, err := provider.FetchUserInfo(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if profile.ProviderUID != "union-1" || profile.DisplayName != "月离用户" || profile.Email != "" || profile.EmailVerified {
		t.Fatalf("profile = %+v", profile)
	}
}
