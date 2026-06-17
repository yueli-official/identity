package oidc

import (
	"net/url"
	"testing"
	"time"

	"github.com/ory/fosite"
)

func TestStoredRequestRoundTrip(t *testing.T) {
	orig := &fosite.Request{
		ID:             "req-1",
		RequestedAt:    time.Now().UTC().Truncate(time.Second),
		Client:         &fosite.DefaultClient{ID: "demo-web"},
		RequestedScope: fosite.Arguments{"openid", "offline_access"},
		GrantedScope:   fosite.Arguments{"openid"},
		Form:           url.Values{"redirect_uri": {"http://127.0.0.1/cb"}},
		Session:        NewSession("iss", "sub-1", "demo-web", "kid", nil, nil, time.Now().UTC()),
	}
	orig.Session.(*Session).IdPSessionID = "sess-9"

	blob, err := marshalRequest(orig)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalRequest(blob, func(id string) (fosite.Client, error) {
		return &fosite.DefaultClient{ID: id}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetClient().GetID() != "demo-web" {
		t.Errorf("client id = %q", got.GetClient().GetID())
	}
	if got.GetSession().GetSubject() != "sub-1" {
		t.Errorf("subject = %q", got.GetSession().GetSubject())
	}
	if got.GetSession().(*Session).IdPSessionID != "sess-9" {
		t.Errorf("IdPSessionID lost: %q", got.GetSession().(*Session).IdPSessionID)
	}
	if got.GetRequestForm().Get("redirect_uri") != "http://127.0.0.1/cb" {
		t.Errorf("form lost")
	}
}
