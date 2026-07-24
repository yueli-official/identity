package oidc_test

import (
	"testing"
	"time"

	"platform/services/identity/internal/authentication"
	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
)

func TestUserinfoScopeGating(t *testing.T) {
	id := model.Identity{ID: "u1", Email: "a@b.com", EmailVerified: true}
	p := model.Profile{DisplayName: "A", Username: "alice", AvatarURL: "x", Locale: "zh-CN"}

	full := oidc.Userinfo(id, p, []string{"openid", "profile", "email", "roles"}, []string{"user", "admin"})
	if full["sub"] != "u1" {
		t.Fatalf("sub = %v", full["sub"])
	}
	if full["email"] != "a@b.com" {
		t.Fatalf("email = %v", full["email"])
	}
	if full["name"] != "A" {
		t.Fatalf("name = %v", full["name"])
	}
	if _, ok := full["roles"]; !ok {
		t.Fatal("roles missing under roles scope")
	}

	// Without email scope, no email key.
	noEmail := oidc.Userinfo(id, p, []string{"openid", "profile"}, nil)
	if _, ok := noEmail["email"]; ok {
		t.Fatal("email leaked without email scope")
	}
	// Without profile scope, no name key.
	if _, ok := noEmail["name"]; !ok {
		t.Fatal("name missing under profile scope")
	}
	bare := oidc.Userinfo(id, p, []string{"openid"}, nil)
	if _, ok := bare["name"]; ok {
		t.Fatal("name leaked without profile scope")
	}
	if len(bare) != 1 { // only sub
		t.Fatalf("bare userinfo has extra keys: %v", bare)
	}
}

func TestBuildSession_RolesClaim(t *testing.T) {
	s := oidc.BuildSession("iss", "cli", "kid", "sid",
		model.Identity{ID: "u1"}, model.Profile{},
		[]string{"openid", "roles"}, []string{"user", "admin"},
		authentication.Password("event-1", time.Now()), time.Now())

	// Access-token JWT extras live on JWTClaims.Extra.
	got := s.JWTClaims.Extra["roles"]
	if r, ok := got.([]string); !ok || len(r) != 2 {
		t.Fatalf("access-token roles claim = %v", got)
	}
	// ID-token extras live on DefaultSession.Claims.Extra.
	if r, ok := s.DefaultSession.Claims.Extra["roles"].([]string); !ok || len(r) != 2 {
		t.Fatalf("id-token roles claim = %v", s.DefaultSession.Claims.Extra["roles"])
	}
}

func TestBuildSessionCarriesAccessTokenClientID(t *testing.T) {
	s := oidc.BuildSession("iss", "blog-ai-web", "kid", "sid",
		model.Identity{ID: "u1"}, model.Profile{},
		[]string{"openid"}, nil,
		authentication.Password("event-1", time.Now()), time.Now())
	if got := s.JWTClaims.Extra["client_id"]; got != "blog-ai-web" {
		t.Fatalf("access-token client_id = %#v, want blog-ai-web", got)
	}
}

func TestBuildSession_NoRolesScope_NoClaim(t *testing.T) {
	s := oidc.BuildSession("iss", "cli", "kid", "sid",
		model.Identity{ID: "u1"}, model.Profile{},
		[]string{"openid"}, []string{"user"},
		authentication.Password("event-1", time.Now()), time.Now())
	if _, ok := s.JWTClaims.Extra["roles"]; ok {
		t.Fatal("roles claim must be absent when scope not granted")
	}
	if _, ok := s.DefaultSession.Claims.Extra["roles"]; ok {
		t.Fatal("id-token roles claim must be absent when scope not granted")
	}
}

func TestBuildSessionSubjectAndClaims(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	id := model.Identity{ID: "u1", Email: "a@b.com", EmailVerified: true}
	p := model.Profile{DisplayName: "A", Username: "alice", Locale: "zh-CN"}
	authenticatedAt := now.Add(-time.Hour)
	s := oidc.BuildSession(
		"iss", "client1", "kid1", "", id, p,
		[]string{"openid", "email", "roles"}, []string{"user"},
		authentication.Password("event-1", authenticatedAt), now,
	)
	if s.GetSubject() != "u1" {
		t.Fatalf("subject = %q", s.GetSubject())
	}
	if !s.DefaultSession.Claims.AuthTime.Equal(authenticatedAt) {
		t.Fatalf("auth_time = %s, want %s", s.DefaultSession.Claims.AuthTime, authenticatedAt)
	}
	if got := s.DefaultSession.Claims.AuthenticationMethodsReferences; len(got) != 1 || got[0] != "pwd" {
		t.Fatalf("amr = %v", got)
	}
	if got := s.DefaultSession.Claims.AuthenticationContextClassReference; got != string(authentication.ProfileBaseline) {
		t.Fatalf("acr = %q", got)
	}
	if got := s.JWTClaims.Extra["auth_time"]; got != authenticatedAt.Unix() {
		t.Fatalf("access auth_time = %#v", got)
	}
}

func TestBuildSessionCarriesIdPSessionID(t *testing.T) {
	id := model.Identity{ID: "id-1", Email: "a@b.com", EmailVerified: true}
	p := model.Profile{DisplayName: "A"}
	s := oidc.BuildSession("iss", "client-1", "kid-1", "sess-xyz", id, p,
		[]string{"openid", "offline_access"}, nil,
		authentication.Password("event-1", time.Now().UTC()), time.Now().UTC())
	if s.IdPSessionID != "sess-xyz" {
		t.Fatalf("IdPSessionID = %q, want sess-xyz", s.IdPSessionID)
	}
}
