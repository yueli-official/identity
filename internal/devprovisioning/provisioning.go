// Package devprovisioning owns the local-only declaration, validation and
// transactional reconciliation of Identity fixtures and Consumer OIDC clients.
package devprovisioning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/lib/pq"
	"github.com/yueli-official/foundation/go/identifier"
	"github.com/yueli-official/identity/internal/user"
	"golang.org/x/crypto/bcrypt"
)

type Mode uint8

const (
	FullSeed Mode = iota + 1
	ClientsOnly
)

type Declaration struct {
	Account        *Account        `json:"account,omitempty"`
	SiteClients    []SiteClient    `json:"siteClients,omitempty"`
	ServiceClients []ServiceClient `json:"serviceClients,omitempty"`
}

type Account struct {
	ID          string `json:"id"`
	UserKey     string `json:"userKey"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type SiteClient struct {
	ID                     string   `json:"id"`
	RedirectURIs           []string `json:"redirectUris"`
	PostLogoutRedirectURIs []string `json:"postLogoutRedirectUris"`
	Audiences              []string `json:"audiences"`
}

type ServiceClient struct {
	ID        string   `json:"id"`
	Secret    string   `json:"secret"`
	SecretRef string   `json:"secretRef"`
	Audience  string   `json:"audience"`
	Scopes    []string `json:"scopes"`
}

type Result struct {
	AccountEmail   string
	SiteClients    int
	ServiceClients int
}

func Parse(raw, source string, mode Mode) (Declaration, error) {
	var declared Declaration
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declared); err != nil {
		return Declaration{}, fmt.Errorf("decode %s: %w", source, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Declaration{}, fmt.Errorf("decode %s: multiple JSON values are not allowed", source)
		}
		return Declaration{}, fmt.Errorf("decode %s: %w", source, err)
	}
	switch mode {
	case FullSeed:
		if declared.Account == nil {
			return Declaration{}, fmt.Errorf("%s requires an account fixture", source)
		}
	case ClientsOnly:
		if declared.Account != nil {
			return Declaration{}, fmt.Errorf("%s must contain only siteClients and serviceClients", source)
		}
		if len(declared.SiteClients)+len(declared.ServiceClients) == 0 {
			return Declaration{}, fmt.Errorf("%s requires at least one OIDC client", source)
		}
	default:
		return Declaration{}, fmt.Errorf("unsupported development provisioning mode %d", mode)
	}
	if declared.Account != nil {
		if err := validateAccount(declared.Account); err != nil {
			return Declaration{}, err
		}
	}
	if err := validateClients(&declared); err != nil {
		return Declaration{}, err
	}
	return declared, nil
}

func Reconcile(ctx context.Context, db *sql.DB, declared Declaration) (Result, error) {
	if db == nil {
		return Result{}, fmt.Errorf("development provisioning database is required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	result := Result{
		SiteClients: len(declared.SiteClients), ServiceClients: len(declared.ServiceClients),
	}
	if declared.Account != nil {
		if err := reconcileAccount(ctx, tx, *declared.Account); err != nil {
			return Result{}, err
		}
		result.AccountEmail = declared.Account.Email
	}
	for _, client := range declared.SiteClients {
		if err := reconcileSiteClient(ctx, tx, client); err != nil {
			return Result{}, err
		}
	}
	for _, client := range declared.ServiceClients {
		if err := reconcileServiceClient(ctx, tx, client); err != nil {
			return Result{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateAccount(account *Account) error {
	if strings.TrimSpace(account.ID) == "" ||
		strings.TrimSpace(account.UserKey) == "" ||
		!strings.Contains(account.Email, "@") ||
		len(account.Password) < 12 ||
		strings.TrimSpace(account.DisplayName) == "" {
		return fmt.Errorf("account UUIDv7, userKey, email, handle, displayName and a password of at least 12 characters are required")
	}
	parsedID, err := identifier.Parse(strings.TrimSpace(account.ID))
	if err != nil || parsedID.Version() != 7 {
		return fmt.Errorf("account id must be a canonical UUIDv7")
	}
	if _, err := user.ParsePublicKey(strings.TrimSpace(account.UserKey)); err != nil {
		return fmt.Errorf("account userKey: %w", err)
	}
	handle, err := user.NormalizeHandle(account.Handle)
	if err != nil {
		return fmt.Errorf("account handle: %w", err)
	}
	account.ID = strings.TrimSpace(account.ID)
	account.UserKey = strings.TrimSpace(account.UserKey)
	account.Email = strings.TrimSpace(account.Email)
	account.Handle = string(handle)
	account.DisplayName = strings.TrimSpace(account.DisplayName)
	return nil
}

func validateClients(declared *Declaration) error {
	seen := map[string]bool{}
	for index := range declared.SiteClients {
		client := &declared.SiteClients[index]
		client.ID = strings.TrimSpace(client.ID)
		if client.ID == "" || seen[client.ID] || len(client.RedirectURIs) == 0 {
			return fmt.Errorf("site client ids must be unique and each client needs a redirect URI")
		}
		seen[client.ID] = true
		client.PostLogoutRedirectURIs = trimNonEmpty(client.PostLogoutRedirectURIs)
		for _, value := range append(append([]string{}, client.RedirectURIs...), client.PostLogoutRedirectURIs...) {
			parsed, err := url.Parse(value)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
				parsed.User != nil || parsed.Fragment != "" {
				return fmt.Errorf("site client %q has invalid redirect URI %q", client.ID, value)
			}
		}
		client.Audiences = trimNonEmpty(client.Audiences)
	}
	for index := range declared.ServiceClients {
		client := &declared.ServiceClients[index]
		client.ID = strings.TrimSpace(client.ID)
		client.SecretRef = strings.TrimSpace(client.SecretRef)
		client.Audience = strings.TrimSpace(client.Audience)
		client.Scopes = trimNonEmpty(client.Scopes)
		if client.ID == "" || seen[client.ID] || len(client.Secret) < 24 || client.SecretRef == "" ||
			client.Audience == "" || len(client.Scopes) == 0 {
			return fmt.Errorf("service clients need a unique id, secret, secretRef, audience and scopes")
		}
		seen[client.ID] = true
	}
	return nil
}

func trimNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func reconcileAccount(ctx context.Context, tx *sql.Tx, account Account) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(account.Password), 12)
	if err != nil {
		return fmt.Errorf("hash development password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO identities (id, user_key, email, email_verified, status)
		VALUES ($1, $2, $3, TRUE, 'active')
		ON CONFLICT (id) DO UPDATE SET
			user_key=EXCLUDED.user_key, email=EXCLUDED.email, email_verified=TRUE, status='active'
		WHERE identities.user_key=EXCLUDED.user_key`, account.ID, account.UserKey, account.Email); err != nil {
		return fmt.Errorf("reconcile development identity: %w", err)
	}
	var storedUserKey string
	if err := tx.QueryRowContext(ctx, `SELECT user_key FROM identities WHERE id=$1`, account.ID).Scan(&storedUserKey); err != nil {
		return fmt.Errorf("verify development public key: %w", err)
	}
	if storedUserKey != account.UserKey {
		return fmt.Errorf("reconcile development identity: userKey is immutable (stored=%s declared=%s)", storedUserKey, account.UserKey)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO oidc_subjects (identity_id, sector_key, subject, subject_type)
		VALUES ($1, 'public', $2, 'public')
		ON CONFLICT (identity_id, sector_key) DO UPDATE SET
			subject=EXCLUDED.subject, subject_type='public'`, account.ID, account.UserKey); err != nil {
		return fmt.Errorf("reconcile development public subject: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_handle_history
		SET state='retired', retired_at=now()
		WHERE identity_id=$1 AND state='current' AND handle<>$2`, account.ID, account.Handle); err != nil {
		return fmt.Errorf("retire previous development handle: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO user_handle_history (handle, identity_id, state)
		VALUES ($1, $2, 'current')
		ON CONFLICT (handle) DO UPDATE SET state='current', retired_at=NULL
		WHERE user_handle_history.identity_id=EXCLUDED.identity_id`, account.Handle, account.ID)
	if err != nil {
		return fmt.Errorf("reconcile development handle: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("reconcile development handle: handle is reserved by another user")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_profiles (identity_id, handle, display_name, locale, bio, social_links)
		VALUES ($1, $2, $3, 'zh-CN', '', '[]'::jsonb)
		ON CONFLICT (identity_id) DO UPDATE SET handle=EXCLUDED.handle, display_name=EXCLUDED.display_name`,
		account.ID, account.Handle, account.DisplayName); err != nil {
		return fmt.Errorf("reconcile development profile: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credentials_password (identity_id, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (identity_id) DO UPDATE SET password_hash=EXCLUDED.password_hash`,
		account.ID, string(passwordHash)); err != nil {
		return fmt.Errorf("reconcile development password: %w", err)
	}
	for _, role := range []string{"user", "admin"} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_roles (identity_id, role_slug)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, account.ID, role); err != nil {
			return fmt.Errorf("reconcile development role %s: %w", role, err)
		}
	}
	return nil
}

func reconcileSiteClient(ctx context.Context, tx *sql.Tx, client SiteClient) error {
	result, err := tx.ExecContext(ctx, `INSERT INTO oidc_clients (
			id, secret_hash, secret_ref, public, redirect_uris, post_logout_redirect_uris,
			audiences, grant_types, response_types, scopes, managed_by
		) VALUES ($1, '', '', TRUE, $2, $3, $4,
			'{authorization_code,refresh_token}', '{code}',
			'{openid,profile,email,roles,offline_access}', 'workspace')
		ON CONFLICT (id) DO UPDATE SET
			secret_hash='', secret_ref='', public=TRUE,
			redirect_uris=EXCLUDED.redirect_uris,
			post_logout_redirect_uris=EXCLUDED.post_logout_redirect_uris,
			audiences=EXCLUDED.audiences,
			grant_types=EXCLUDED.grant_types,
			response_types=EXCLUDED.response_types,
			scopes=EXCLUDED.scopes,
			managed_by='workspace',
			updated_at=now()
		WHERE oidc_clients.managed_by IN ('doctor', 'workspace')`,
		client.ID, pq.Array(client.RedirectURIs), pq.Array(client.PostLogoutRedirectURIs), pq.Array(client.Audiences))
	if err != nil {
		return fmt.Errorf("reconcile site client %s: %w", client.ID, err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("reconcile site client %s: existing client is not owned by local Workspace provisioning", client.ID)
	}
	return nil
}

func reconcileServiceClient(ctx context.Context, tx *sql.Tx, client ServiceClient) error {
	secretHash, err := reusableSecretHash(ctx, tx, client)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO oidc_clients (
			id, secret_hash, secret_ref, public, redirect_uris, post_logout_redirect_uris,
			audiences, grant_types, response_types, scopes, managed_by
		) VALUES ($1, $2, $3, FALSE, '{}', '{}', ARRAY[$4],
			'{client_credentials}', '{}', $5, 'workspace')
		ON CONFLICT (id) DO UPDATE SET
			secret_hash=EXCLUDED.secret_hash,
			secret_ref=EXCLUDED.secret_ref,
			public=FALSE,
			redirect_uris='{}',
			post_logout_redirect_uris='{}',
			audiences=EXCLUDED.audiences,
			grant_types=EXCLUDED.grant_types,
			response_types=EXCLUDED.response_types,
			scopes=EXCLUDED.scopes,
			managed_by='workspace',
			updated_at=now()
		WHERE oidc_clients.managed_by IN ('doctor', 'workspace')`,
		client.ID, secretHash, client.SecretRef, client.Audience, pq.Array(client.Scopes))
	if err != nil {
		return fmt.Errorf("reconcile service client %s: %w", client.ID, err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("reconcile service client %s: existing client is not owned by local Workspace provisioning", client.ID)
	}
	return nil
}

func reusableSecretHash(ctx context.Context, tx *sql.Tx, client ServiceClient) (string, error) {
	var storedHash, managedBy string
	err := tx.QueryRowContext(ctx, `SELECT secret_hash, managed_by FROM oidc_clients WHERE id=$1`, client.ID).
		Scan(&storedHash, &managedBy)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return "", fmt.Errorf("inspect service client %s: %w", client.ID, err)
	case managedBy != "doctor" && managedBy != "workspace":
		return "", fmt.Errorf("reconcile service client %s: existing client is not owned by local Workspace provisioning", client.ID)
	case bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(client.Secret)) == nil:
		return storedHash, nil
	}
	secretHash, err := bcrypt.GenerateFromPassword([]byte(client.Secret), 12)
	if err != nil {
		return "", fmt.Errorf("hash service client %s secret: %w", client.ID, err)
	}
	return string(secretHash), nil
}
