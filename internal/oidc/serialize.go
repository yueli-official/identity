package oidc

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/ory/fosite"
)

// storedRequest is the JSON-serializable projection of a fosite.Requester.
// The client is reduced to its id (re-resolved on load); the session is the
// concrete *Session so it round-trips without interface-decoding gymnastics.
type storedRequest struct {
	ID                string     `json:"id"`
	RequestedAt       time.Time  `json:"requested_at"`
	ClientID          string     `json:"client_id"`
	RequestedScope    []string   `json:"requested_scope"`
	GrantedScope      []string   `json:"granted_scope"`
	RequestedAudience []string   `json:"requested_audience"`
	GrantedAudience   []string   `json:"granted_audience"`
	Form              url.Values `json:"form"`
	Session           *Session   `json:"session"`
}

// clientResolver re-hydrates a fosite.Client from its id (backed by ClientRepo).
type clientResolver func(id string) (fosite.Client, error)

func marshalRequest(r fosite.Requester) ([]byte, error) {
	sess, _ := r.GetSession().(*Session)
	sr := storedRequest{
		ID:                r.GetID(),
		RequestedAt:       r.GetRequestedAt(),
		ClientID:          r.GetClient().GetID(),
		RequestedScope:    []string(r.GetRequestedScopes()),
		GrantedScope:      []string(r.GetGrantedScopes()),
		RequestedAudience: []string(r.GetRequestedAudience()),
		GrantedAudience:   []string(r.GetGrantedAudience()),
		Form:              r.GetRequestForm(),
		Session:           sess,
	}
	return json.Marshal(sr)
}

func unmarshalRequest(blob []byte, resolve clientResolver) (*fosite.Request, error) {
	var sr storedRequest
	if err := json.Unmarshal(blob, &sr); err != nil {
		return nil, err
	}
	client, err := resolve(sr.ClientID)
	if err != nil {
		return nil, err
	}
	form := sr.Form
	if form == nil {
		form = url.Values{}
	}
	var sess fosite.Session
	if sr.Session != nil {
		sess = sr.Session
	}
	return &fosite.Request{
		ID:                sr.ID,
		RequestedAt:       sr.RequestedAt,
		Client:            client,
		RequestedScope:    fosite.Arguments(sr.RequestedScope),
		GrantedScope:      fosite.Arguments(sr.GrantedScope),
		RequestedAudience: fosite.Arguments(sr.RequestedAudience),
		GrantedAudience:   fosite.Arguments(sr.GrantedAudience),
		Form:              form,
		Session:           sess,
	}, nil
}
