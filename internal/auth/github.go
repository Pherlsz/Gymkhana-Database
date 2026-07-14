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
	defaultGitHubAuthorizeURL = "https://github.com/login/oauth/authorize"
	defaultGitHubTokenURL     = "https://github.com/login/oauth/access_token"
	defaultGitHubUserURL      = "https://api.github.com/user"
)

type GitHubProviderOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

type GitHubProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	userURL      string
}

func NewGitHubProvider(options GitHubProviderOptions) (*GitHubProvider, error) {
	if strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.ClientSecret) == "" || strings.TrimSpace(options.RedirectURL) == "" {
		return nil, errors.New("github oauth client id, secret, and redirect url are required")
	}
	if _, err := url.ParseRequestURI(options.RedirectURL); err != nil {
		return nil, fmt.Errorf("parse github oauth redirect url: %w", err)
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &GitHubProvider{
		clientID:     strings.TrimSpace(options.ClientID),
		clientSecret: strings.TrimSpace(options.ClientSecret),
		redirectURL:  strings.TrimSpace(options.RedirectURL),
		httpClient:   httpClient,
		authorizeURL: valueOrDefault(options.AuthorizeURL, defaultGitHubAuthorizeURL),
		tokenURL:     valueOrDefault(options.TokenURL, defaultGitHubTokenURL),
		userURL:      valueOrDefault(options.UserURL, defaultGitHubUserURL),
	}, nil
}

func (provider *GitHubProvider) AuthorizationURL(state string) string {
	query := url.Values{
		"client_id":    {provider.clientID},
		"redirect_uri": {provider.redirectURL},
		"scope":        {"read:user"},
		"state":        {state},
	}
	return provider.authorizeURL + "?" + query.Encode()
}

func (provider *GitHubProvider) Exchange(ctx context.Context, code string) (GitHubIdentity, error) {
	requestBody, err := json.Marshal(map[string]string{
		"client_id":     provider.clientID,
		"client_secret": provider.clientSecret,
		"code":          code,
		"redirect_uri":  provider.redirectURL,
	})
	if err != nil {
		return GitHubIdentity{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.tokenURL, bytes.NewReader(requestBody))
	if err != nil {
		return GitHubIdentity{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := provider.httpClient.Do(request)
	if err != nil {
		return GitHubIdentity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return GitHubIdentity{}, fmt.Errorf("github token exchange returned status %d", response.StatusCode)
	}

	var tokenPayload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := decodeLimitedJSON(response.Body, &tokenPayload); err != nil {
		return GitHubIdentity{}, fmt.Errorf("decode github token response: %w", err)
	}
	if tokenPayload.Error != "" || tokenPayload.AccessToken == "" {
		return GitHubIdentity{}, fmt.Errorf("github token exchange failed: %s", valueOrDefault(tokenPayload.Error, "missing access token"))
	}

	userRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.userURL, nil)
	if err != nil {
		return GitHubIdentity{}, err
	}
	userRequest.Header.Set("Accept", "application/vnd.github+json")
	userRequest.Header.Set("Authorization", "Bearer "+tokenPayload.AccessToken)
	userRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	userResponse, err := provider.httpClient.Do(userRequest)
	if err != nil {
		return GitHubIdentity{}, err
	}
	defer userResponse.Body.Close()
	if userResponse.StatusCode != http.StatusOK {
		return GitHubIdentity{}, fmt.Errorf("github user request returned status %d", userResponse.StatusCode)
	}

	var userPayload struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := decodeLimitedJSON(userResponse.Body, &userPayload); err != nil {
		return GitHubIdentity{}, fmt.Errorf("decode github user response: %w", err)
	}
	return GitHubIdentity{
		UserID:      userPayload.ID,
		Login:       userPayload.Login,
		DisplayName: userPayload.Name,
		AvatarURL:   userPayload.AvatarURL,
	}, nil
}

func decodeLimitedJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	return decoder.Decode(destination)
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
