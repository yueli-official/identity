package oauthlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"platform/gokit/observability"
)

// GoogleProvider implements Provider against Google's OAuth2 + v2 userinfo
// endpoints. Endpoints are overridable (newGoogleWithEndpoints) for hermetic tests.
type GoogleProvider struct {
	clientID, clientSecret, redirectURL string
	authURL, tokenURL, userinfoURL      string
	hc                                  *http.Client
}

// NewGoogle builds a GoogleProvider pointed at the real Google endpoints.
func NewGoogle(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return newGoogleWithEndpoints(clientID, clientSecret, redirectURL,
		"https://accounts.google.com/o/oauth2/v2/auth",
		"https://oauth2.googleapis.com/token",
		"https://www.googleapis.com/oauth2/v2/userinfo")
}

func newGoogleWithEndpoints(clientID, clientSecret, redirectURL, authURL, tokenURL, userinfoURL string) *GoogleProvider {
	return &GoogleProvider{
		clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL,
		authURL: authURL, tokenURL: tokenURL, userinfoURL: userinfoURL,
		hc: observability.HTTPClient(&http.Client{Timeout: 10 * time.Second}),
	}
}

func (g *GoogleProvider) Name() string { return "google" }

func (g *GoogleProvider) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.clientID)
	q.Set("redirect_uri", g.redirectURL)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	q.Set("prompt", "select_account")
	return g.authURL + "?" + q.Encode()
}

// CheckHealth checks the authorization endpoint without exchanging a code or
// creating/linking an identity. Any non-5xx HTTP response proves connectivity;
// credential completeness is represented separately in the manifest.
func (g *GoogleProvider) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.authURL, nil)
	if err != nil {
		return err
	}
	resp, err := g.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		return fmt.Errorf("google authorization endpoint returned %d", resp.StatusCode)
	}
	return nil
}

func (g *GoogleProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", g.clientID)
	form.Set("client_secret", g.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", g.redirectURL)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("google token endpoint %d: %s", resp.StatusCode, string(b))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("google token response missing access_token")
	}
	return tr.AccessToken, nil
}

func (g *GoogleProvider) FetchUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, g.userinfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := g.hc.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return UserInfo{}, fmt.Errorf("google userinfo %d: %s", resp.StatusCode, string(b))
	}
	var ui struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return UserInfo{}, err
	}
	return UserInfo{
		ProviderUID:   ui.ID,
		Email:         ui.Email,
		EmailVerified: ui.VerifiedEmail,
		DisplayName:   ui.Name,
		AvatarURL:     ui.Picture,
	}, nil
}

// compile-time assertion that GoogleProvider satisfies Provider.
var _ Provider = (*GoogleProvider)(nil)
var _ HealthChecker = (*GoogleProvider)(nil)
