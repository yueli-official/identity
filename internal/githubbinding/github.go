package githubbinding

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	identityruntime "platform/services/identity/internal/runtime"
)

const githubAPIVersion = "2026-03-10"

type GitHubApp struct {
	clientID     string
	clientSecret string
	redirectURL  string
	authorizeURL string
	tokenURL     string
	userURL      string
	revokeURL    string
	client       *http.Client
}

func NewGitHubApp(clientID, clientSecret, redirectURL string) (*GitHubApp, error) {
	return newGitHubAppWithEndpoints(
		clientID, clientSecret, redirectURL,
		"https://github.com/login/oauth/authorize",
		"https://github.com/login/oauth/access_token",
		"https://api.github.com/user",
		"https://api.github.com/applications/"+url.PathEscape(clientID)+"/token",
	)
}

func newGitHubAppWithEndpoints(
	clientID, clientSecret, redirectURL, authorizeURL, tokenURL, userURL, revokeURL string,
) (*GitHubApp, error) {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, ErrUnavailable
	}
	return &GitHubApp{
		clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL,
		authorizeURL: authorizeURL, tokenURL: tokenURL, userURL: userURL,
		revokeURL: revokeURL,
		client:    identityruntime.TelemetryHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	}, nil
}

func (provider *GitHubApp) AuthorizationURL(state, codeChallenge string) string {
	query := url.Values{}
	query.Set("client_id", provider.clientID)
	query.Set("redirect_uri", provider.redirectURL)
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	return provider.authorizeURL + "?" + query.Encode()
}

// CheckHealth performs a side-effect-free reachability probe. Any non-5xx
// response proves the authorization endpoint is reachable without minting a
// code or token.
func (provider *GitHubApp) CheckHealth(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.authorizeURL, nil)
	if err != nil {
		return err
	}
	response, err := provider.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("github authorization endpoint returned %d", response.StatusCode)
	}
	return nil
}

func (provider *GitHubApp) ExchangeCode(
	ctx context.Context,
	code string,
	verifier string,
) (string, error) {
	form := url.Values{}
	form.Set("client_id", provider.clientID)
	form.Set("client_secret", provider.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", provider.redirectURL)
	form.Set("code_verifier", verifier)
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, provider.tokenURL, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := provider.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token endpoint returned %d", response.StatusCode)
	}
	var value struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&value); err != nil {
		return "", err
	}
	if value.Error != "" || value.AccessToken == "" {
		return "", fmt.Errorf("github token exchange rejected: %s", value.Error)
	}
	return value.AccessToken, nil
}

func (provider *GitHubApp) AuthenticatedUser(
	ctx context.Context,
	accessToken string,
) (Account, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.userURL, nil)
	if err != nil {
		return Account{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	response, err := provider.client.Do(request)
	if err != nil {
		return Account{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Account{}, fmt.Errorf("github user endpoint returned %d", response.StatusCode)
	}
	var value struct {
		ID        int64  `json:"id"`
		NodeID    string `json:"node_id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&value); err != nil {
		return Account{}, err
	}
	if value.ID <= 0 || strings.TrimSpace(value.Login) == "" {
		return Account{}, ErrProviderFailure
	}
	return Account{
		AccountID: strconv.FormatInt(value.ID, 10), NodeID: value.NodeID,
		Login: value.Login, AvatarURL: value.AvatarURL,
	}, nil
}

func (provider *GitHubApp) RevokeAccessToken(
	ctx context.Context,
	accessToken string,
) error {
	body, _ := json.Marshal(map[string]string{"access_token": accessToken})
	request, err := http.NewRequestWithContext(
		ctx, http.MethodDelete, provider.revokeURL, bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.SetBasicAuth(provider.clientID, provider.clientSecret)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	response, err := provider.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("github token revocation returned %d", response.StatusCode)
	}
	return nil
}

// VerifyWebhookSignature validates GitHub's sha256= HMAC over the unmodified
// request body. It deliberately accepts no SHA-1 fallback.
func VerifyWebhookSignature(secret []byte, body []byte, header string) bool {
	if len(secret) == 0 || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

var _ Provider = (*GitHubApp)(nil)
