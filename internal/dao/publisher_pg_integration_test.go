//go:build integration

package dao_test

import (
	"context"
	"testing"
	"time"

	"github.com/yueli-official/identity/internal/dao"
	"github.com/yueli-official/identity/internal/publisher"
)

func TestPublisherAttestationStoreIsIdempotent(t *testing.T) {
	db := newDB(t)
	store := dao.NewPG(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	value := publisher.Attestation{
		AttestationID:    "810d3c8f-fb22-4a4c-a4ec-a598f26f8180",
		Issuer:           "https://identity.publisher-pg.test",
		PublisherSubject: "e6f841dd-e2a2-42aa-b060-9c67b54f4a6b",
		StatementDigest:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CommandDigest:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		KeyID:            "publisher-key-1", EnvelopeJSON: `{"payloadType":"application/vnd.in-toto+json","payload":"e30=","signatures":[{"keyid":"publisher-key-1","sig":"AA=="}]}`,
		IssuedAt: now, IdempotencyKey: "publisher-pg-idempotency-0001",
		Audience: "urn:yueli:registry:yotta", ConsumerInstance: "urn:yueli:platform-instance:test",
		Namespace: "example",
		Artifact: publisher.Artifact{
			Kind: "workflow-release", Identity: "example/tool", Version: "1.0.0",
			Name: "workflow:example/tool@1.0.0", URI: "pkg:yueli-workflow/example/tool@1.0.0",
			MediaType: "application/vnd.yueli.workflow-release.v1+json",
			SHA256:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
	}
	defer func() {
		_, _ = db.Exec(ctx, "DELETE FROM publisher_attestations WHERE issuer = ?", value.Issuer)
	}()

	first, inserted, err := store.PutIfAbsent(ctx, value)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || first.AttestationID != value.AttestationID {
		t.Fatalf("first PutIfAbsent() = %#v, %t", first, inserted)
	}

	value.AttestationID = "33cf7774-aabc-4835-8b5c-7909c88f055a"
	second, inserted, err := store.PutIfAbsent(ctx, value)
	if err != nil {
		t.Fatal(err)
	}
	if inserted || second.AttestationID != first.AttestationID {
		t.Fatalf("second PutIfAbsent() = %#v, %t", second, inserted)
	}

	got, ok, err := store.GetByIdempotency(ctx, value.Issuer, value.PublisherSubject, value.IdempotencyKey)
	if err != nil || !ok || got.CommandDigest != value.CommandDigest {
		t.Fatalf("GetByIdempotency() = %#v, %t, %v", got, ok, err)
	}
}

var _ publisher.Store = (*dao.PG)(nil)
