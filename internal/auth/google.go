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
	defaultGoogleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL     = "https://oauth2.googleapis.com/token"
	defaultGoogleUserURL      = "https://www.googleapis.com/oauth2/v2/userinfo"
)

type GoogleProviderOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	userURL      string
}

func NewGoogleProvider(options GoogleProviderOptions) (*GoogleProvider, error) {
	if strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.ClientSecret) == "" || strings.TrimSpace(options.RedirectURL) == "" {
		return nil, errors.New("google oauth client id, secret, and redirect url are required")
	}
	if _, err := url.ParseRequestURI(options.RedirectURL); err != nil {
		return nil, fmt.Errorf("parse google oauth redirect url: %w", err)
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &GoogleProvider{
		clientID:     strings.TrimSpace(options.ClientID),
		clientSecret: strings.TrimSpace(options.ClientSecret),
		redirectURL:  strings.TrimSpace(options.RedirectURL),
		httpClient:   httpClient,
		authorizeURL: valueOrDefault(options.AuthorizeURL, defaultGoogleAuthorizeURL),
		tokenURL:     valueOrDefault(options.TokenURL, defaultGoogleTokenURL),
		userURL:      valueOrDefault(options.UserURL, defaultGoogleUserURL),
	}, nil
}

func (provider *GoogleProvider) AuthorizationURL(state string) string {
	query := url.Values{
		"client_id":     {provider.clientID},
		"redirect_uri":  {provider.redirectURL},
		"response_type": {"code"},
		"scope":         {"email profile"},
		"state":         {state},
		"access_type":   {"offline"},
	}
	return provider.authorizeURL + "?" + query.Encode()
}

func (provider *GoogleProvider) Exchange(ctx context.Context, code string) (GoogleIdentity, error) {
	requestBody, err := json.Marshal(map[string]string{
		"client_id":     provider.clientID,
		"client_secret": provider.clientSecret,
		"code":          code,
		"redirect_uri":  provider.redirectURL,
		"grant_type":    "authorization_code",
	})
	if err != nil {
		return GoogleIdentity{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.tokenURL, bytes.NewReader(requestBody))
	if err != nil {
		return GoogleIdentity{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := provider.httpClient.Do(request)
	if err != nil {
		return GoogleIdentity{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return GoogleIdentity{}, fmt.Errorf("google token exchange returned status %d", response.StatusCode)
	}

	var tokenPayload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := decodeLimitedJSON(response.Body, &tokenPayload); err != nil {
		return GoogleIdentity{}, fmt.Errorf("decode google token response: %w", err)
	}
	if tokenPayload.Error != "" || tokenPayload.AccessToken == "" {
		return GoogleIdentity{}, fmt.Errorf("google token exchange failed: %s", valueOrDefault(tokenPayload.Error, "missing access token"))
	}

	userRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.userURL, nil)
	if err != nil {
		return GoogleIdentity{}, err
	}
	userRequest.Header.Set("Accept", "application/json")
	userRequest.Header.Set("Authorization", "Bearer "+tokenPayload.AccessToken)

	userResponse, err := provider.httpClient.Do(userRequest)
	if err != nil {
		return GoogleIdentity{}, err
	}
	defer userResponse.Body.Close()
	if userResponse.StatusCode != http.StatusOK {
		return GoogleIdentity{}, fmt.Errorf("google user request returned status %d", userResponse.StatusCode)
	}

	var userPayload struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := decodeLimitedJSON(userResponse.Body, &userPayload); err != nil {
		return GoogleIdentity{}, fmt.Errorf("decode google user response: %w", err)
	}
	return GoogleIdentity{
		Subject:     userPayload.ID,
		Email:       userPayload.Email,
		DisplayName: userPayload.Name,
		AvatarURL:   userPayload.Picture,
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
