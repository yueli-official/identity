package logic_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/actor"
	"github.com/yueli-official/identity/internal/iderr"
	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/repo"
)

func TestRegister_GrantsDefaultRole(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	id, err := s.Register(context.Background(), logic.RegisterInput{
		Email: "d@example.com", Password: "correct horse battery", DisplayName: "D",
	})
	if err != nil {
		t.Fatal(err)
	}
	roles, err := s.GetRoles(context.Background(), id.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != logic.DefaultRole {
		t.Fatalf("want [user], got %v", roles)
	}
}

func TestGrantRevokeRole(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	ctx := context.Background()
	id, err := s.Register(ctx, logic.RegisterInput{
		Email: "g@example.com", Password: "correct horse battery", DisplayName: "G",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	roles, err := s.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("want user+admin, got %v", roles)
	}
	if err := s.RevokeRole(ctx, id.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	roles, err = s.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != logic.DefaultRole {
		t.Fatalf("want [user] after revoke, got %v", roles)
	}
}

func TestRevokeRole_RejectsRemovingOwnAdminRole(t *testing.T) {
	m := repo.NewMemory()
	s := logic.New(m, logic.DefaultConfig())
	ctx := context.Background()
	id, err := s.Register(ctx, logic.RegisterInput{
		Email: "self-admin@example.com", Password: "correct horse battery", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantRole(ctx, id.ID, logic.AdminRole); err != nil {
		t.Fatal(err)
	}

	err = s.RevokeRole(actor.WithIdentity(ctx, id.ID), id.ID, logic.AdminRole)
	if codeOf(t, err) != iderr.CodeSelfAdminActionForbidden {
		t.Fatalf("want CodeSelfAdminActionForbidden, got %v", err)
	}

	roles, err := s.GetRoles(ctx, id.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("self-admin role must remain assigned, got %v", roles)
	}
}
