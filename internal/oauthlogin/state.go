package oauthlogin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type statePayload struct {
	RT    string `json:"rt"`
	Nonce string `json:"n"`
	Bind  string `json:"b,omitempty"` // identity id to LINK to (bind flow); "" = login flow
	Exp   int64  `json:"e"`
}

// ErrBadState marks any state token that fails signature, decode, or expiry checks.
var ErrBadState = errors.New("invalid oauth state")

// EncodeState signs (returnTo, nonce, bind, exp) into a CSRF-safe state token
// using HMAC-SHA256 over the base64url-encoded payload. bind is the identity id
// the callback should LINK the provider to (account-binding flow); pass "" for
// the normal login/register flow.
func EncodeState(secret []byte, returnTo, nonce, bind string, exp int64) string {
	p, _ := json.Marshal(statePayload{RT: returnTo, Nonce: nonce, Bind: bind, Exp: exp})
	body := base64.RawURLEncoding.EncodeToString(p)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig
}

// DecodeState verifies the signature and expiry of a state token, returning the
// embedded returnTo, nonce, and bind identity id. Any failure yields ErrBadState.
func DecodeState(secret []byte, token string) (returnTo, nonce, bind string, err error) {
	body, sig, ok := cut(token, '.')
	if !ok {
		return "", "", "", ErrBadState
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", "", "", ErrBadState
	}
	raw, derr := base64.RawURLEncoding.DecodeString(body)
	if derr != nil {
		return "", "", "", ErrBadState
	}
	var p statePayload
	if json.Unmarshal(raw, &p) != nil {
		return "", "", "", ErrBadState
	}
	if time.Now().Unix() > p.Exp {
		return "", "", "", ErrBadState
	}
	return p.RT, p.Nonce, p.Bind, nil
}

func cut(s string, sep byte) (a, b string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}
