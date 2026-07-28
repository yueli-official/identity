// Command devseed reconciles Identity-owned local development fixtures.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type seed struct {
	Account        account         `json:"account"`
	SiteClients    []siteClient    `json:"siteClients"`
	ServiceClients []serviceClient `json:"serviceClients"`
}

type account struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type siteClient struct {
	ID                     string   `json:"id"`
	RedirectURIs           []string `json:"redirectUris"`
	PostLogoutRedirectURIs []string `json:"postLogoutRedirectUris"`
	Audiences              []string `json:"audiences"`
}

type serviceClient struct {
	ID        string   `json:"id"`
	Secret    string   `json:"secret"`
	SecretRef string   `json:"secretRef"`
	Audience  string   `json:"audience"`
	Scopes    []string `json:"scopes"`
}

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("IDENTITY_DATABASE_URL"))
	raw := strings.TrimSpace(os.Getenv("IDENTITY_DEV_SEED"))
	if databaseURL == "" || raw == "" {
		fatal("IDENTITY_DATABASE_URL and IDENTITY_DEV_SEED are required")
	}
	declared, err := parseSeed(raw)
	if err != nil {
		fatal("%v", err)
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		fatal("open database: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		fatal("connect database: %v", err)
	}
	if err := reconcile(ctx, db, declared); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("identity development seed reconciled (account=%s, siteClients=%d)\n", declared.Account.Email, len(declared.SiteClients))
}

func parseSeed(raw string) (seed, error) {
	var declared seed
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declared); err != nil {
		return seed{}, fmt.Errorf("decode IDENTITY_DEV_SEED: %w", err)
	}
	if strings.TrimSpace(declared.Account.ID) == "" ||
		!strings.Contains(declared.Account.Email, "@") ||
		len(declared.Account.Password) < 12 ||
		strings.TrimSpace(declared.Account.DisplayName) == "" {
		return seed{}, fmt.Errorf("account id, email, displayName and a password of at least 12 characters are required")
	}
	seen := map[string]bool{}
	for _, client := range declared.SiteClients {
		if strings.TrimSpace(client.ID) == "" || seen[client.ID] || len(client.RedirectURIs) == 0 {
			return seed{}, fmt.Errorf("site client ids must be unique and each client needs a redirect URI")
		}
		seen[client.ID] = true
		for _, value := range append(append([]string{}, client.RedirectURIs...), client.PostLogoutRedirectURIs...) {
			parsed, err := url.Parse(value)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return seed{}, fmt.Errorf("site client %q has invalid redirect URI %q", client.ID, value)
			}
		}
	}
	for _, client := range declared.ServiceClients {
		if strings.TrimSpace(client.ID) == "" || seen[client.ID] ||
			len(client.Secret) < 24 || strings.TrimSpace(client.SecretRef) == "" ||
			strings.TrimSpace(client.Audience) == "" || len(client.Scopes) == 0 {
			return seed{}, fmt.Errorf("service clients need a unique id, secret, secretRef, audience and scopes")
		}
		seen[client.ID] = true
	}
	return declared, nil
}

func reconcile(ctx context.Context, db *sql.DB, declared seed) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(declared.Account.Password), 12)
	if err != nil {
		return fmt.Errorf("hash development password: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO identities (id, email, email_verified, status)
		VALUES ($1, $2, TRUE, 'active')
		ON CONFLICT (id) DO UPDATE SET email=EXCLUDED.email, email_verified=TRUE, status='active'`,
		declared.Account.ID, declared.Account.Email); err != nil {
		return fmt.Errorf("reconcile development identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_profiles (identity_id, display_name, locale, bio, social_links)
		VALUES ($1, $2, 'zh-CN', '', '[]'::jsonb)
		ON CONFLICT (identity_id) DO UPDATE SET display_name=EXCLUDED.display_name`,
		declared.Account.ID, declared.Account.DisplayName); err != nil {
		return fmt.Errorf("reconcile development profile: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO credentials_password (identity_id, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (identity_id) DO UPDATE SET password_hash=EXCLUDED.password_hash`,
		declared.Account.ID, string(passwordHash)); err != nil {
		return fmt.Errorf("reconcile development password: %w", err)
	}
	for _, role := range []string{"user", "admin"} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO identity_roles (identity_id, role_slug)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`, declared.Account.ID, role); err != nil {
			return fmt.Errorf("reconcile development role %s: %w", role, err)
		}
	}
	for _, client := range declared.SiteClients {
		if _, err := tx.ExecContext(ctx, `INSERT INTO oidc_clients (
				id, secret_hash, secret_ref, public, redirect_uris, post_logout_redirect_uris,
				audiences, grant_types, response_types, scopes, managed_by
			) VALUES ($1, '', '', TRUE, $2, $3, $4,
				'{authorization_code,refresh_token}', '{code}',
				'{openid,profile,email,roles,offline_access}', 'doctor')
			ON CONFLICT (id) DO UPDATE SET
				secret_hash='', secret_ref='', public=TRUE,
				redirect_uris=EXCLUDED.redirect_uris,
				post_logout_redirect_uris=EXCLUDED.post_logout_redirect_uris,
				audiences=EXCLUDED.audiences,
				grant_types=EXCLUDED.grant_types,
				response_types=EXCLUDED.response_types,
				scopes=EXCLUDED.scopes,
				managed_by='doctor',
				updated_at=now()`,
			client.ID, pq.Array(client.RedirectURIs), pq.Array(client.PostLogoutRedirectURIs), pq.Array(client.Audiences)); err != nil {
			return fmt.Errorf("reconcile site client %s: %w", client.ID, err)
		}
	}
	for _, client := range declared.ServiceClients {
		secretHash, err := bcrypt.GenerateFromPassword([]byte(client.Secret), 12)
		if err != nil {
			return fmt.Errorf("hash service client %s secret: %w", client.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO oidc_clients (
				id, secret_hash, secret_ref, public, redirect_uris, audiences,
				grant_types, response_types, scopes, managed_by
			) VALUES ($1, $2, $3, FALSE, '{}', ARRAY[$4],
				'{client_credentials}', '{}', $5, 'doctor')
			ON CONFLICT (id) DO UPDATE SET
				secret_hash=EXCLUDED.secret_hash,
				secret_ref=EXCLUDED.secret_ref,
				public=FALSE,
				redirect_uris='{}',
				audiences=EXCLUDED.audiences,
				grant_types=EXCLUDED.grant_types,
				response_types=EXCLUDED.response_types,
				scopes=EXCLUDED.scopes,
				managed_by='doctor',
				updated_at=now()`,
			client.ID, string(secretHash), client.SecretRef, client.Audience, pq.Array(client.Scopes)); err != nil {
			return fmt.Errorf("reconcile service client %s: %w", client.ID, err)
		}
	}
	return tx.Commit()
}

func fatal(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, "devseed: "+format+"\n", arguments...)
	os.Exit(1)
}
