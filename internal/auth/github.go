package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint         = "https://oauth2.googleapis.com/token"
	googleUserInfoEndpoint      = "https://openidconnect.googleapis.com/v1/userinfo"
)

var ErrProviderExchange = errors.New("oauth provider exchange failed")

type GoogleProviderOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client

	// Endpoint overrides are restricted to tests. Production leaves them empty.
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
}

// Compatibility aliases keep existing composition code source-compatible while
// the public configuration and documentation move from GitHub to Google.
type GitHubProviderOptions = GoogleProviderOptions

type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	userInfoURL  string
}

type GitHubProvider = GoogleProvider

func NewGoogleProvider(options GoogleProviderOptions) (*GoogleProvider, error) {
	if strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.ClientSecret) == "" || strings.TrimSpace(options.RedirectURL) == "" {
		return nil, errors.New("google oauth client id, secret and redirect url are required")
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	authorizeURL := strings.TrimSpace(options.AuthorizeURL)
	if authorizeURL == "" {
		authorizeURL = googleAuthorizationEndpoint
	}
	tokenURL := strings.TrimSpace(options.TokenURL)
	if tokenURL == "" {
		tokenURL = googleTokenEndpoint
	}
	userInfoURL := strings.TrimSpace(options.UserInfoURL)
	if userInfoURL == "" {
		userInfoURL = googleUserInfoEndpoint
	}
	return &GoogleProvider{
		clientID:     strings.TrimSpace(options.ClientID),
		clientSecret: strings.TrimSpace(options.ClientSecret),
		redirectURL:  strings.TrimSpace(options.RedirectURL),
		httpClient:   client,
		authorizeURL: authorizeURL,
		tokenURL:     tokenURL,
		userInfoURL:  userInfoURL,
	}, nil
}

func NewGitHubProvider(options GitHubProviderOptions) (*GoogleProvider, error) {
	return NewGoogleProvider(options)
}

func (provider *GoogleProvider) AuthorizationURL(state string) string {
	values := url.Values{}
	values.Set("client_id", provider.clientID)
	values.Set("redirect_uri", provider.redirectURL)
	values.Set("response_type", "code")
	values.Set("scope", "openid email profile")
	values.Set("state", state)
	values.Set("prompt", "select_account")
	values.Set("include_granted_scopes", "true")
	return provider.authorizeURL + "?" + values.Encode()
}

func (provider *GoogleProvider) Exchange(ctx context.Context, code string) (GoogleIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return GoogleIdentity{}, ErrInvalidOAuthCode
	}
	form := url.Values{}
	form.Set("client_id", provider.clientID)
	form.Set("client_secret", provider.clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", provider.redirectURL)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: create token request: %v", ErrProviderExchange, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := provider.httpClient.Do(request)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: token request: %v", ErrProviderExchange, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: read token response: %v", ErrProviderExchange, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GoogleIdentity{}, fmt.Errorf("%w: token endpoint status %d", ErrProviderExchange, response.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &token); err != nil || strings.TrimSpace(token.AccessToken) == "" {
		return GoogleIdentity{}, fmt.Errorf("%w: invalid token response", ErrProviderExchange)
	}

	userinfoRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.userInfoURL, nil)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: create userinfo request: %v", ErrProviderExchange, err)
	}
	userinfoRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	userinfoResponse, err := provider.httpClient.Do(userinfoRequest)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: userinfo request: %v", ErrProviderExchange, err)
	}
	defer userinfoResponse.Body.Close()
	userinfoBody, err := io.ReadAll(io.LimitReader(userinfoResponse.Body, 1<<20))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: read userinfo response: %v", ErrProviderExchange, err)
	}
	if userinfoResponse.StatusCode < 200 || userinfoResponse.StatusCode >= 300 {
		return GoogleIdentity{}, fmt.Errorf("%w: userinfo endpoint status %d", ErrProviderExchange, userinfoResponse.StatusCode)
	}
	var userinfo struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(userinfoBody, &userinfo); err != nil {
		return GoogleIdentity{}, fmt.Errorf("%w: decode userinfo response: %v", ErrProviderExchange, err)
	}
	if strings.TrimSpace(userinfo.Subject) == "" || strings.TrimSpace(userinfo.Email) == "" || !userinfo.EmailVerified {
		return GoogleIdentity{}, fmt.Errorf("%w: google identity is missing a verified email", ErrProviderExchange)
	}
	return GoogleIdentity{
		Subject:       userinfo.Subject,
		Email:         userinfo.Email,
		EmailVerified: true,
		DisplayName:   userinfo.Name,
		AvatarURL:     userinfo.Picture,
	}, nil
}
