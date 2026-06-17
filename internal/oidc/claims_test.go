package oidc_test

import (
	"testing"
	"time"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/oidc"
)

func TestUserinfoScopeGating(t *testing.T) {
	id := model.Identity{ID: "u1", Email: "a@b.com", EmailVerified: true}
	p := model.Profile{DisplayName: "A", Username: "alice", AvatarURL: "x", Locale: "zh-CN"}

	full := oidc.Userinfo(id, p, []string{"openid", "profile", "email", "roles"})
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
	noEmail := oidc.Userinfo(id, p, []string{"openid", "profile"})
	if _, ok := noEmail["email"]; ok {
		t.Fatal("email leaked without email scope")
	}
	// Without profile scope, no name key.
	if _, ok := noEmail["name"]; !ok {
		t.Fatal("name missing under profile scope")
	}
	bare := oidc.Userinfo(id, p, []string{"openid"})
	if _, ok := bare["name"]; ok {
		t.Fatal("name leaked without profile scope")
	}
	if len(bare) != 1 { // only sub
		t.Fatalf("bare userinfo has extra keys: %v", bare)
	}
}

func TestBuildSessionSubjectAndClaims(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	id := model.Identity{ID: "u1", Email: "a@b.com", EmailVerified: true}
	p := model.Profile{DisplayName: "A", Username: "alice", Locale: "zh-CN"}
	s := oidc.BuildSession("iss", "client1", "kid1", id, p, []string{"openid", "email", "roles"}, now)
	if s.GetSubject() != "u1" {
		t.Fatalf("subject = %q", s.GetSubject())
	}
}
