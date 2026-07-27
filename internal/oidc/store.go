package oidc

import (
	"context"
	"errors"
	"time"

	"github.com/ory/fosite"

	"github.com/yueli-official/identity/internal/model"
	"github.com/yueli-official/identity/internal/repo"
)

// Store adapts our Backend + ClientRepo to fosite's storage interfaces.
// Transient sessions persist via Backend (PG in prod, memory in tests). Access
// tokens are NOT stored (JWT self-contained) — those methods no-op.
type Store struct {
	be      Backend
	clients repo.ClientRepo
}

func NewStore(be Backend, clients repo.ClientRepo) *Store {
	return &Store{be: be, clients: clients}
}

// resolveClient re-hydrates a client by id. clientResolver is ctx-less, so
// callers bind the request ctx in a closure (preserves deadline/cancellation
// and request-scoped tracing values).
func (s *Store) resolveClient(ctx context.Context, id string) (fosite.Client, error) {
	c, err := s.clients.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}
	return toFositeClient(c), nil
}

// ── ClientManager ────────────────────────────────────────────────────────────

func (s *Store) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	c, err := s.clients.GetClient(ctx, id)
	if err != nil {
		return nil, fosite.ErrNotFound.WithWrap(err)
	}
	return toFositeClient(c), nil
}

// We don't use client-JWT assertions (public clients + PKCE). Accept all / store
// nothing — these satisfy fosite.ClientManager.
func (s *Store) ClientAssertionJWTValid(ctx context.Context, jti string) error { return nil }
func (s *Store) SetClientAssertionJWT(ctx context.Context, jti string, exp time.Time) error {
	return nil
}

func toFositeClient(c model.OIDCClient) *fosite.DefaultClient {
	// Confidential clients (e.g. a service using client_credentials) carry a
	// bcrypt secret hash; fosite compares the presented secret against it with
	// the default BCrypt hasher. Public clients (PKCE) keep Secret nil.
	var secret []byte
	if !c.Public && c.SecretHash != "" {
		secret = []byte(c.SecretHash)
	}
	return &fosite.DefaultClient{
		ID:            c.ID,
		Public:        c.Public,
		Secret:        secret,
		RedirectURIs:  c.RedirectURIs,
		GrantTypes:    c.GrantTypes,
		ResponseTypes: c.ResponseTypes,
		Scopes:        c.Scopes,
		Audience:      c.Audiences,
	}
}

// ── generic helpers (authcode/pkce/oidc) ─────────────────────────────────────

func (s *Store) putGeneric(ctx context.Context, kind, sig string, req fosite.Requester) error {
	blob, err := marshalRequest(req)
	if err != nil {
		return err
	}
	return s.be.PutGeneric(ctx, kind, sig, Record{
		RequestID: req.GetID(),
		ClientID:  req.GetClient().GetID(),
		Subject:   subjectOf(req),
		Active:    true,
		ExpiresAt: req.GetSession().GetExpiresAt(fosite.AuthorizeCode),
		Data:      blob,
	})
}

func (s *Store) getGeneric(ctx context.Context, kind, sig string) (fosite.Requester, bool, error) {
	rec, err := s.be.GetGeneric(ctx, kind, sig)
	if errors.Is(err, ErrBackendNotFound) {
		return nil, false, fosite.ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	req, err := unmarshalRequest(rec.Data, func(id string) (fosite.Client, error) { return s.resolveClient(ctx, id) })
	if err != nil {
		return nil, false, err
	}
	return req, rec.Active, nil
}

func subjectOf(req fosite.Requester) string {
	if sess, ok := req.GetSession().(*Session); ok {
		return sess.GetSubject()
	}
	return ""
}

// ── AuthorizeCodeStorage ─────────────────────────────────────────────────────

func (s *Store) CreateAuthorizeCodeSession(ctx context.Context, code string, req fosite.Requester) error {
	return s.putGeneric(ctx, kindAuthCode, code, req)
}

func (s *Store) GetAuthorizeCodeSession(ctx context.Context, code string, _ fosite.Session) (fosite.Requester, error) {
	req, active, err := s.getGeneric(ctx, kindAuthCode, code)
	if err != nil {
		return nil, err
	}
	if !active {
		// fosite REQUIRES the requester alongside this error.
		return req, fosite.ErrInvalidatedAuthorizeCode
	}
	return req, nil
}

func (s *Store) InvalidateAuthorizeCodeSession(ctx context.Context, code string) error {
	return s.be.DeactivateGeneric(ctx, kindAuthCode, code)
}

// ── AccessTokenStorage (no-op: JWT self-contained) ───────────────────────────

func (s *Store) CreateAccessTokenSession(ctx context.Context, sig string, req fosite.Requester) error {
	return nil
}
func (s *Store) GetAccessTokenSession(ctx context.Context, sig string, _ fosite.Session) (fosite.Requester, error) {
	return nil, fosite.ErrNotFound
}
func (s *Store) DeleteAccessTokenSession(ctx context.Context, sig string) error { return nil }

// ── RefreshTokenStorage ──────────────────────────────────────────────────────

func (s *Store) CreateRefreshTokenSession(ctx context.Context, sig, accessSig string, req fosite.Requester) error {
	blob, err := marshalRequest(req)
	if err != nil {
		return err
	}
	sessID := ""
	if sess, ok := req.GetSession().(*Session); ok {
		sessID = sess.IdPSessionID
	}
	return s.be.PutRefresh(ctx, sig, RefreshRecord{
		RequestID:       req.GetID(),
		ClientID:        req.GetClient().GetID(),
		Subject:         subjectOf(req),
		SessionID:       sessID,
		AccessSignature: accessSig,
		Active:          true,
		ExpiresAt:       req.GetSession().GetExpiresAt(fosite.RefreshToken),
		Data:            blob,
	})
}

func (s *Store) GetRefreshTokenSession(ctx context.Context, sig string, _ fosite.Session) (fosite.Requester, error) {
	rec, err := s.be.GetRefresh(ctx, sig)
	if errors.Is(err, ErrBackendNotFound) {
		return nil, fosite.ErrNotFound
	}
	if errors.Is(err, ErrBackendInactive) {
		req, derr := unmarshalRequest(rec.Data, func(id string) (fosite.Client, error) { return s.resolveClient(ctx, id) })
		if derr != nil {
			return nil, derr
		}
		return req, fosite.ErrInactiveToken
	}
	if err != nil {
		return nil, err
	}
	return unmarshalRequest(rec.Data, func(id string) (fosite.Client, error) { return s.resolveClient(ctx, id) })
}

func (s *Store) DeleteRefreshTokenSession(ctx context.Context, sig string) error {
	return s.be.DeleteRefresh(ctx, sig)
}

func (s *Store) RotateRefreshToken(ctx context.Context, requestID, sig string) error {
	// MVP: no grace period — old refresh becomes immediately inactive.
	return s.be.DeactivateRefresh(ctx, sig)
}

// ── TokenRevocationStorage ───────────────────────────────────────────────────

func (s *Store) RevokeRefreshToken(ctx context.Context, requestID string) error {
	return s.be.RevokeRefreshByRequestID(ctx, requestID)
}

func (s *Store) RevokeAccessToken(ctx context.Context, requestID string) error {
	// Access tokens are stateless JWTs; immediate revocation is a denylist (later).
	return nil
}

// ── PKCERequestStorage ───────────────────────────────────────────────────────

func (s *Store) CreatePKCERequestSession(ctx context.Context, sig string, req fosite.Requester) error {
	return s.putGeneric(ctx, kindPKCE, sig, req)
}

func (s *Store) GetPKCERequestSession(ctx context.Context, sig string, _ fosite.Session) (fosite.Requester, error) {
	req, _, err := s.getGeneric(ctx, kindPKCE, sig)
	return req, err
}

func (s *Store) DeletePKCERequestSession(ctx context.Context, sig string) error {
	return s.be.DeleteGeneric(ctx, kindPKCE, sig)
}

// ── OpenIDConnectRequestStorage ──────────────────────────────────────────────

func (s *Store) CreateOpenIDConnectSession(ctx context.Context, code string, req fosite.Requester) error {
	return s.putGeneric(ctx, kindOIDC, code, req)
}

func (s *Store) GetOpenIDConnectSession(ctx context.Context, code string, _ fosite.Requester) (fosite.Requester, error) {
	req, _, err := s.getGeneric(ctx, kindOIDC, code)
	return req, err // not-found already mapped to fosite.ErrNotFound (== openid.ErrNoSessionFound)
}

func (s *Store) DeleteOpenIDConnectSession(ctx context.Context, code string) error {
	return s.be.DeleteGeneric(ctx, kindOIDC, code)
}

// ── Transactional (delegated to Backend) ─────────────────────────────────────

func (s *Store) BeginTX(ctx context.Context) (context.Context, error) { return s.be.BeginTX(ctx) }
func (s *Store) Commit(ctx context.Context) error                     { return s.be.Commit(ctx) }
func (s *Store) Rollback(ctx context.Context) error                   { return s.be.Rollback(ctx) }

// ── session-bound revocation (consumed by logic.RefreshRevoker) ──────────────

func (s *Store) RevokeRefreshBySession(ctx context.Context, sessionID string) error {
	return s.be.RevokeRefreshBySession(ctx, sessionID)
}

func (s *Store) RevokeRefreshByIdentity(ctx context.Context, subject string) error {
	return s.be.RevokeRefreshByIdentity(ctx, subject)
}
