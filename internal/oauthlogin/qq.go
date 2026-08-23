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

	identityruntime "github.com/yueli-official/identity/internal/runtime"
)

type QQProvider struct {
	clientID, clientSecret, redirectURL string
	authURL, tokenURL, meURL, userURL   string
	hc                                  *http.Client
}

func NewQQ(clientID, clientSecret, redirectURL string) *QQProvider {
	return newQQWithEndpoints(
		clientID, clientSecret, redirectURL,
		"https://graph.qq.com/oauth2.0/authorize",
		"https://graph.qq.com/oauth2.0/token",
		"https://graph.qq.com/oauth2.0/me",
		"https://graph.qq.com/user/get_user_info",
	)
}

func newQQWithEndpoints(clientID, clientSecret, redirectURL, authURL, tokenURL, meURL, userURL string) *QQProvider {
	return &QQProvider{
		clientID: clientID, clientSecret: clientSecret, redirectURL: redirectURL,
		authURL: authURL, tokenURL: tokenURL, meURL: meURL, userURL: userURL,
		hc: identityruntime.TelemetryHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	}
}

func (q *QQProvider) Name() string { return "qq" }

func (q *QQProvider) AuthorizeURL(state string) string {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", q.clientID)
	values.Set("redirect_uri", q.redirectURL)
	values.Set("scope", "get_user_info")
	values.Set("state", state)
	return q.authURL + "?" + values.Encode()
}

func (q *QQProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", q.clientID)
	values.Set("client_secret", q.clientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", q.redirectURL)
	values.Set("fmt", "json")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, q.tokenURL+"?"+values.Encode(), nil)
	if err != nil {
		return "", err
	}
	response, err := q.hc.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qq token endpoint %d: %s", response.StatusCode, string(body))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		Error       int    `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	parsed, parseErr := url.ParseQuery(string(body))
	if parseErr == nil && parsed.Get("access_token") != "" {
		return parsed.Get("access_token"), nil
	}
	return "", fmt.Errorf("qq token response missing access_token: %s", payload.ErrorDesc)
}

func (q *QQProvider) FetchUserInfo(ctx context.Context, accessToken string) (UserInfo, error) {
	identity, err := q.fetchIdentity(ctx, accessToken)
	if err != nil {
		return UserInfo{}, err
	}
	providerUID := identity.UnionID
	if providerUID == "" {
		providerUID = identity.OpenID
	}
	if providerUID == "" {
		return UserInfo{}, fmt.Errorf("qq identity response missing openid")
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	values.Set("oauth_consumer_key", q.clientID)
	values.Set("openid", identity.OpenID)
	values.Set("fmt", "json")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, q.userURL+"?"+values.Encode(), nil)
	if err != nil {
		return UserInfo{}, err
	}
	response, err := q.hc.Do(request)
	if err != nil {
		return UserInfo{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return UserInfo{}, fmt.Errorf("qq userinfo endpoint %d: %s", response.StatusCode, string(body))
	}
	var profile struct {
		Ret          int    `json:"ret"`
		Message      string `json:"msg"`
		Nickname     string `json:"nickname"`
		FigureURLQQ2 string `json:"figureurl_qq_2"`
		FigureURLQQ1 string `json:"figureurl_qq_1"`
	}
	if err := json.NewDecoder(response.Body).Decode(&profile); err != nil {
		return UserInfo{}, err
	}
	if profile.Ret != 0 {
		return UserInfo{}, fmt.Errorf("qq userinfo failed: %s", profile.Message)
	}
	avatar := profile.FigureURLQQ2
	if avatar == "" {
		avatar = profile.FigureURLQQ1
	}
	return UserInfo{
		ProviderUID: providerUID,
		DisplayName: profile.Nickname,
		AvatarURL:   avatar,
	}, nil
}

type qqIdentity struct {
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
	Error   int    `json:"error"`
	Message string `json:"error_description"`
}

func (q *QQProvider) fetchIdentity(ctx context.Context, accessToken string) (qqIdentity, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	values.Set("fmt", "json")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, q.meURL+"?"+values.Encode(), nil)
	if err != nil {
		return qqIdentity{}, err
	}
	response, err := q.hc.Do(request)
	if err != nil {
		return qqIdentity{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return qqIdentity{}, err
	}
	if response.StatusCode != http.StatusOK {
		return qqIdentity{}, fmt.Errorf("qq identity endpoint %d: %s", response.StatusCode, string(body))
	}
	body = []byte(strings.TrimSpace(string(body)))
	if strings.HasPrefix(string(body), "callback(") {
		body = []byte(strings.TrimSuffix(strings.TrimPrefix(string(body), "callback("), ");"))
	}
	var identity qqIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		return qqIdentity{}, err
	}
	if identity.Error != 0 {
		return qqIdentity{}, fmt.Errorf("qq identity failed: %s", identity.Message)
	}
	return identity, nil
}

func (q *QQProvider) CheckHealth(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, q.authURL, nil)
	if err != nil {
		return err
	}
	response, err := q.hc.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("qq authorization endpoint returned %d", response.StatusCode)
	}
	return nil
}

var _ Provider = (*QQProvider)(nil)
var _ HealthChecker = (*QQProvider)(nil)
