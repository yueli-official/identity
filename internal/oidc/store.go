package oidc

import (
	"context"

	"github.com/ory/fosite"
	"github.com/ory/fosite/storage"

	"platform/services/identity/internal/model"
	"platform/services/identity/internal/repo"
)

// Store satisfies fosite's storage needs. Transient OAuth sessions (auth codes,
// PKCE, access-token sessions, OIDC sessions) live in fosite's in-memory store
// (milestone ③ scope; milestone ④ persists them to PG/Redis). Only GetClient is
// overridden to read registered clients from PG (repo.ClientRepo).
type Store struct {
	*storage.MemoryStore
	clients repo.ClientRepo
}

// Compile-time assertion: Store satisfies fosite.Storage (= ClientManager).
var _ fosite.Storage = (*Store)(nil)

// NewStore builds the adapter around a fresh in-memory session store + a client repo.
func NewStore(clients repo.ClientRepo) *Store {
	return &Store{MemoryStore: storage.NewMemoryStore(), clients: clients}
}

// GetClient reads a registered client from the client repo (shadows the embedded
// MemoryStore's GetClient).
func (s *Store) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	c, err := s.clients.GetClient(ctx, id)
	if err != nil {
		return nil, fosite.ErrNotFound.WithWrap(err)
	}
	return toFositeClient(c), nil
}

func toFositeClient(c model.OIDCClient) *fosite.DefaultClient {
	return &fosite.DefaultClient{
		ID:            c.ID,
		Public:        c.Public,
		Secret:        nil, // public clients only in ③; confidential support is later
		RedirectURIs:  c.RedirectURIs,
		GrantTypes:    c.GrantTypes,
		ResponseTypes: c.ResponseTypes,
		Scopes:        c.Scopes,
	}
}
