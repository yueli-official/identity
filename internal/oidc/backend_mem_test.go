package oidc

import (
	"context"
	"testing"
	"time"
)

func TestMemBackendGenericRoundTrip(t *testing.T) {
	ctx := context.Background()
	be := newMemBackend()
	rec := Record{RequestID: "r1", ClientID: "c1", Subject: "s1", Active: true, Data: []byte(`{"x":1}`)}
	if err := be.PutGeneric(ctx, "authcode", "sig1", rec); err != nil {
		t.Fatal(err)
	}
	got, err := be.GetGeneric(ctx, "authcode", "sig1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != `{"x":1}` || !got.Active {
		t.Fatalf("bad record: %+v", got)
	}
	if err := be.DeactivateGeneric(ctx, "authcode", "sig1"); err != nil {
		t.Fatal(err)
	}
	got, _ = be.GetGeneric(ctx, "authcode", "sig1")
	if got.Active {
		t.Fatal("expected inactive after deactivate")
	}
	if _, err := be.GetGeneric(ctx, "authcode", "missing"); err != ErrBackendNotFound {
		t.Fatalf("want ErrBackendNotFound, got %v", err)
	}
}

func TestMemBackendRefreshRotateRevoke(t *testing.T) {
	ctx := context.Background()
	be := newMemBackend()
	put := func(sig, reqID, sess, sub string) {
		if err := be.PutRefresh(ctx, sig, RefreshRecord{
			RequestID: reqID, ClientID: "c1", Subject: sub, SessionID: sess,
			Active: true, ExpiresAt: time.Now().Add(time.Hour), Data: []byte("{}"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("rt1", "req-A", "sess-1", "sub-1")

	if err := be.DeactivateRefresh(ctx, "rt1"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt1"); err != ErrBackendInactive {
		t.Fatalf("want ErrBackendInactive, got %v", err)
	}

	put("rt2", "req-B", "sess-1", "sub-1")
	put("rt3", "req-B", "sess-1", "sub-1")
	if err := be.RevokeRefreshByRequestID(ctx, "req-B"); err != nil {
		t.Fatal(err)
	}
	for _, sig := range []string{"rt2", "rt3"} {
		if _, err := be.GetRefresh(ctx, sig); err == nil {
			t.Fatalf("%s should be gone/inactive", sig)
		}
	}

	put("rt4", "req-C", "sess-2", "sub-2")
	put("rt5", "req-D", "sess-2", "sub-2")
	if err := be.RevokeRefreshBySession(ctx, "sess-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt4"); err == nil {
		t.Fatal("rt4 should be revoked by session")
	}

	put("rt6", "req-E", "sess-3", "sub-9")
	if err := be.RevokeRefreshBySubject(ctx, "sub-9"); err != nil {
		t.Fatal(err)
	}
	if _, err := be.GetRefresh(ctx, "rt6"); err == nil {
		t.Fatal("rt6 should be revoked by identity")
	}
}
