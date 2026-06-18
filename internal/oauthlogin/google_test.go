package oauthlogin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGoogle_ExchangeAndFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("code") != "good-code" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3599}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sub-1","email":"u@example.com","verified_email":true,"name":"U","picture":"http://p"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := newGoogleWithEndpoints("cid", "secret", "http://localhost/cb",
		srv.URL+"/auth", srv.URL+"/token", srv.URL+"/userinfo")

	at, err := g.ExchangeCode(context.Background(), "good-code")
	if err != nil || at != "AT" {
		t.Fatalf("exchange: %q %v", at, err)
	}
	ui, err := g.FetchUserInfo(context.Background(), at)
	if err != nil {
		t.Fatal(err)
	}
	if ui.ProviderUID != "sub-1" || ui.Email != "u@example.com" || !ui.EmailVerified || ui.DisplayName != "U" {
		t.Fatalf("userinfo mismatch: %+v", ui)
	}
}

func TestGoogle_AuthorizeURL(t *testing.T) {
	g := NewGoogle("cid", "secret", "http://localhost/cb")
	u, _ := url.Parse(g.AuthorizeURL("STATE"))
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != "STATE" ||
		q.Get("response_type") != "code" || q.Get("redirect_uri") != "http://localhost/cb" {
		t.Fatalf("bad authorize url: %s", u.String())
	}
}
