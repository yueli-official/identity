package catalog

import (
	"testing"

	"github.com/yueli-official/foundation/go/privacy"
)

func TestOwnerContractsCompileWithStableDigestsAndIdentityFinalizer(t *testing.T) {
	definition := privacy.Definition{
		Version: privacy.DefinitionVersion, Consumer: "privacy-catalog-test",
		SubjectKinds: SubjectKinds(), DataCategories: Categories(),
		RetentionRules: RetentionRules(),
		Coordination: &privacy.CoordinationDefinition{
			Owners: Owners(Notification(), Blog()), RightsPolicies: RightsPolicies(),
		},
	}
	first, err := privacy.Compile(definition)
	if err != nil {
		t.Fatal(err)
	}
	second, err := privacy.Compile(definition)
	if err != nil {
		t.Fatal(err)
	}
	owners := first.Owners()
	if len(owners) != 3 {
		t.Fatalf("owners = %d, want 3", len(owners))
	}
	for index, owner := range owners {
		if owner.Ref.Digest == "" || owner.Ref.Digest != second.Owners()[index].Ref.Digest {
			t.Fatalf("unstable owner digest for %q", owner.Ref.Key)
		}
		if owner.Ref.Key == IdentityOwner && !owner.FinalizeAfterOwners {
			t.Fatal("identity owner must finalize after product owners")
		}
	}
}
