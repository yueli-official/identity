package logic_test

import (
	"context"
	"testing"

	"github.com/yueli-official/identity/internal/logic"
	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

func TestOIDCSubjectAssignmentSeparatesPublicAndPairwiseScopes(t *testing.T) {
	ctx := context.Background()
	store := repo.NewMemory()
	service := logic.New(store, logic.DefaultConfig())
	identity, err := service.Register(ctx, logic.RegisterInput{
		Email: "subject@example.com", Password: "correct horse battery", DisplayName: "Subject",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	public, err := service.OIDCSubject(ctx, identity.ID, model.OIDCClient{SubjectType: "public"})
	if err != nil {
		t.Fatalf("public subject: %v", err)
	}
	if public != identity.UserKey {
		t.Fatalf("public subject = %q, want public user key %q", public, identity.UserKey)
	}

	pairwiseA, err := service.OIDCSubject(ctx, identity.ID, model.OIDCClient{
		SubjectType: "pairwise", SubjectSector: "third-party.example",
	})
	if err != nil {
		t.Fatalf("pairwise subject: %v", err)
	}
	pairwiseAAgain, _ := service.OIDCSubject(ctx, identity.ID, model.OIDCClient{
		SubjectType: "pairwise", SubjectSector: "third-party.example",
	})
	pairwiseB, _ := service.OIDCSubject(ctx, identity.ID, model.OIDCClient{
		SubjectType: "pairwise", SubjectSector: "other.example",
	})
	if pairwiseA != pairwiseAAgain || pairwiseA == pairwiseB || pairwiseA == public {
		t.Fatalf("unexpected subject assignments: public=%q A=%q A2=%q B=%q", public, pairwiseA, pairwiseAAgain, pairwiseB)
	}
	resolved, err := service.GetByOIDCSubject(ctx, pairwiseA)
	if err != nil || resolved.ID != identity.ID {
		t.Fatalf("resolve pairwise subject: identity=%+v err=%v", resolved, err)
	}
}
