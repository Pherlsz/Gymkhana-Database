package googleforms

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maximumProviderResponseBytes int64 = 4 << 20

type OAuthToken struct {
	RefreshToken string
	Scopes       []string
}

type Provider interface {
	AuthorizationURL(state, codeChallenge string) (string, error)
	Exchange(context.Context, string, string) (OAuthToken, error)
	Revoke(context.Context, string) error
	GetForm(context.Context, string, string) (Form, error)
	ListResponses(context.Context, string, string, *time.Time, string, int) (ResponsePage, error)
}

type ProviderError struct {
	Code       string
	Retryable  bool
	RetryAfter time.Duration
}

func (value *ProviderError) Error() string {
	if value == nil || value.Code == "" {
		return "google forms provider error"
	}
	return "google forms provider error: " + value.Code
}

func (value *ProviderError) Unwrap() []error {
	if value == nil {
		return nil
	}
	errorsList := []error{ErrProvider}
	if value.Retryable {
		errorsList = append(errorsList, ErrProviderRetryable)
	}
	if value.Code == "rate_limited" {
		errorsList = append(errorsList, ErrRateLimited)
	}
	if value.Code == "invalid_grant" || value.Code == "authorization_failed" {
		errorsList = append(errorsList, ErrNeedsReauth)
	}
	return errorsList
}

type GoogleProviderOptions struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
	AuthorizeURL string
	TokenURL     string
	RevokeURL    string
	FormsBaseURL string
}

type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
	authorizeURL string
	tokenURL     string
	revokeURL    string
	formsBaseURL string
}

func NewGoogleProvider(options GoogleProviderOptions) (*GoogleProvider, error) {
	if strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.ClientSecret) == "" || strings.TrimSpace(options.RedirectURL) == "" {
		return nil, ErrInvalidInput
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 20 * time.Second}
	}
	if options.AuthorizeURL == "" {
		options.AuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if options.TokenURL == "" {
		options.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if options.RevokeURL == "" {
		options.RevokeURL = "https://oauth2.googleapis.com/revoke"
	}
	if options.FormsBaseURL == "" {
		options.FormsBaseURL = "https://forms.googleapis.com/v1"
	}
	for _, endpoint := range []string{options.AuthorizeURL, options.TokenURL, options.RevokeURL, options.FormsBaseURL} {
		parsed, err := url.Parse(endpoint)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" {
			return nil, ErrInvalidInput
		}
	}
	return &GoogleProvider{
		clientID: strings.TrimSpace(options.ClientID), clientSecret: strings.TrimSpace(options.ClientSecret),
		redirectURL: strings.TrimSpace(options.RedirectURL), httpClient: options.HTTPClient,
		authorizeURL: strings.TrimSuffix(options.AuthorizeURL, "/"), tokenURL: options.TokenURL,
		revokeURL: options.RevokeURL, formsBaseURL: strings.TrimSuffix(options.FormsBaseURL, "/"),
	}, nil
}

func (provider *GoogleProvider) AuthorizationURL(state, codeChallenge string) (string, error) {
	if provider == nil || strings.TrimSpace(state) == "" || strings.TrimSpace(codeChallenge) == "" {
		return "", ErrInvalidInput
	}
	endpoint, err := url.Parse(provider.authorizeURL)
	if err != nil {
		return "", ErrInvalidState
	}
	query := endpoint.Query()
	query.Set("client_id", provider.clientID)
	query.Set("redirect_uri", provider.redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(RequiredScopes, " "))
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func (provider *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier string) (OAuthToken, error) {
	if provider == nil || strings.TrimSpace(code) == "" || strings.TrimSpace(codeVerifier) == "" {
		return OAuthToken{}, ErrInvalidInput
	}
	values := url.Values{
		"client_id": {provider.clientID}, "client_secret": {provider.clientSecret},
		"code": {code}, "code_verifier": {codeVerifier}, "grant_type": {"authorization_code"},
		"redirect_uri": {provider.redirectURL},
	}
	var payload tokenResponse
	if err := provider.postForm(ctx, provider.tokenURL, values, &payload); err != nil {
		return OAuthToken{}, err
	}
	if strings.TrimSpace(payload.RefreshToken) == "" {
		return OAuthToken{}, &ProviderError{Code: "refresh_token_missing"}
	}
	return OAuthToken{RefreshToken: payload.RefreshToken, Scopes: strings.Fields(payload.Scope)}, nil
}

func (provider *GoogleProvider) Revoke(ctx context.Context, refreshToken string) error {
	if provider == nil || strings.TrimSpace(refreshToken) == "" {
		return ErrInvalidInput
	}
	return provider.postForm(ctx, provider.revokeURL, url.Values{"token": {refreshToken}}, nil)
}

func (provider *GoogleProvider) GetForm(ctx context.Context, refreshToken, formID string) (Form, error) {
	if !validProviderFormID(formID) {
		return Form{}, ErrInvalidInput
	}
	accessToken, err := provider.accessToken(ctx, refreshToken)
	if err != nil {
		return Form{}, err
	}
	var payload providerForm
	endpoint := provider.formsBaseURL + "/forms/" + url.PathEscape(formID)
	if err := provider.getJSON(ctx, endpoint, accessToken, &payload); err != nil {
		return Form{}, err
	}
	return normalizeProviderForm(payload)
}

func (provider *GoogleProvider) ListResponses(ctx context.Context, refreshToken, formID string, after *time.Time, pageToken string, pageSize int) (ResponsePage, error) {
	if !validProviderFormID(formID) || pageSize < 1 || pageSize > 500 || len(pageToken) > 2048 {
		return ResponsePage{}, ErrInvalidInput
	}
	accessToken, err := provider.accessToken(ctx, refreshToken)
	if err != nil {
		return ResponsePage{}, err
	}
	endpoint, err := url.Parse(provider.formsBaseURL + "/forms/" + url.PathEscape(formID) + "/responses")
	if err != nil {
		return ResponsePage{}, ErrInvalidState
	}
	query := endpoint.Query()
	query.Set("pageSize", strconv.Itoa(pageSize))
	if after != nil {
		query.Set("filter", "timestamp >= "+after.UTC().Format(time.RFC3339Nano))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	endpoint.RawQuery = query.Encode()
	var payload providerResponses
	if err := provider.getJSON(ctx, endpoint.String(), accessToken, &payload); err != nil {
		return ResponsePage{}, err
	}
	return normalizeProviderResponses(payload)
}

func (provider *GoogleProvider) accessToken(ctx context.Context, refreshToken string) (string, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return "", ErrNeedsReauth
	}
	values := url.Values{
		"client_id": {provider.clientID}, "client_secret": {provider.clientSecret},
		"refresh_token": {refreshToken}, "grant_type": {"refresh_token"},
	}
	var payload tokenResponse
	if err := provider.postForm(ctx, provider.tokenURL, values, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", &ProviderError{Code: "access_token_missing"}
	}
	return payload.AccessToken, nil
}

func (provider *GoogleProvider) postForm(ctx context.Context, endpoint string, values url.Values, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return ErrProvider
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return provider.do(request, destination)
}

func (provider *GoogleProvider) getJSON(ctx context.Context, endpoint, accessToken string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrProvider
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	return provider.do(request, destination)
}

func (provider *GoogleProvider) do(request *http.Request, destination any) error {
	response, err := provider.httpClient.Do(request)
	if err != nil {
		return &ProviderError{Code: "network_error", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return classifyProviderResponse(response)
	}
	if destination == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(response.Body, maximumProviderResponseBytes))
		return err
	}
	limited := &io.LimitedReader{R: response.Body, N: maximumProviderResponseBytes + 1}
	decoder := json.NewDecoder(limited)
	if err := decoder.Decode(destination); err != nil {
		return &ProviderError{Code: "invalid_response"}
	}
	if limited.N <= 0 {
		return &ProviderError{Code: "response_too_large"}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return &ProviderError{Code: "invalid_response"}
	}
	return nil
}

func classifyProviderResponse(response *http.Response) error {
	providerCode := "provider_failed"
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&body)
	if len(body.Error) > 0 {
		var code string
		if body.Error[0] == '"' {
			_ = json.Unmarshal(body.Error, &code)
		} else {
			var nested struct {
				Status string `json:"status"`
				Code   int    `json:"code"`
			}
			_ = json.Unmarshal(body.Error, &nested)
			code = strings.ToLower(strings.TrimSpace(nested.Status))
		}
		if code != "" {
			providerCode = code
		}
	}
	switch response.StatusCode {
	case http.StatusBadRequest:
		if providerCode == "invalid_grant" {
			return &ProviderError{Code: "invalid_grant"}
		}
		return &ProviderError{Code: "invalid_request"}
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Code: "authorization_failed"}
	case http.StatusNotFound:
		return &ProviderError{Code: "form_not_found"}
	case http.StatusTooManyRequests:
		return &ProviderError{Code: "rate_limited", Retryable: true, RetryAfter: retryAfter(response.Header.Get("Retry-After"))}
	default:
		if response.StatusCode >= 500 {
			return &ProviderError{Code: "provider_unavailable", Retryable: true, RetryAfter: retryAfter(response.Header.Get("Retry-After"))}
		}
		return &ProviderError{Code: providerCode}
	}
}

func retryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return min(time.Duration(seconds)*time.Second, 5*time.Minute)
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return min(max(time.Until(parsed), 0), 5*time.Minute)
	}
	return 0
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type providerForm struct {
	FormID     string `json:"formId"`
	RevisionID string `json:"revisionId"`
	Info       struct {
		Title string `json:"title"`
	} `json:"info"`
	Items []providerItem `json:"items"`
}

type providerItem struct {
	ItemID            string                `json:"itemId"`
	Title             string                `json:"title"`
	QuestionItem      *providerQuestionItem `json:"questionItem"`
	QuestionGroupItem json.RawMessage       `json:"questionGroupItem"`
}

type providerQuestionItem struct {
	Question providerQuestion `json:"question"`
}

type providerQuestion struct {
	QuestionID         string          `json:"questionId"`
	Required           bool            `json:"required"`
	ChoiceQuestion     json.RawMessage `json:"choiceQuestion"`
	TextQuestion       json.RawMessage `json:"textQuestion"`
	ScaleQuestion      json.RawMessage `json:"scaleQuestion"`
	DateQuestion       json.RawMessage `json:"dateQuestion"`
	TimeQuestion       json.RawMessage `json:"timeQuestion"`
	FileUploadQuestion json.RawMessage `json:"fileUploadQuestion"`
}

type providerResponses struct {
	Responses     []providerResponse `json:"responses"`
	NextPageToken string             `json:"nextPageToken"`
}

type providerResponse struct {
	ResponseID        string                    `json:"responseId"`
	CreateTime        string                    `json:"createTime"`
	LastSubmittedTime string                    `json:"lastSubmittedTime"`
	Answers           map[string]providerAnswer `json:"answers"`
}

type providerAnswer struct {
	TextAnswers *struct {
		Answers []struct {
			Value string `json:"value"`
		} `json:"answers"`
	} `json:"textAnswers"`
}

func normalizeProviderForm(payload providerForm) (Form, error) {
	if !validProviderFormID(payload.FormID) || strings.TrimSpace(payload.RevisionID) == "" || strings.TrimSpace(payload.Info.Title) == "" || len(payload.Items) > MaximumQuestions {
		return Form{}, &ProviderError{Code: "invalid_form_schema"}
	}
	questions := make([]Question, 0, len(payload.Items))
	seen := make(map[string]struct{}, len(payload.Items))
	for _, item := range payload.Items {
		if item.QuestionItem == nil {
			if len(item.QuestionGroupItem) == 0 {
				continue
			}
			id := strings.TrimSpace(item.ItemID)
			if id == "" {
				return Form{}, &ProviderError{Code: "invalid_form_schema"}
			}
			if _, duplicate := seen[id]; duplicate {
				return Form{}, &ProviderError{Code: "duplicate_question"}
			}
			seen[id] = struct{}{}
			question := Question{ID: id, Position: len(questions), Title: normalizedQuestionTitle(item.Title, len(questions)), AnswerKind: AnswerUnsupported, Supported: false, UnsupportedCode: "question_group"}
			question.QuestionFingerprint = fingerprintQuestion(question)
			questions = append(questions, question)
			continue
		}
		value := item.QuestionItem.Question
		id := strings.TrimSpace(value.QuestionID)
		if id == "" {
			return Form{}, &ProviderError{Code: "invalid_form_schema"}
		}
		if _, duplicate := seen[id]; duplicate {
			return Form{}, &ProviderError{Code: "duplicate_question"}
		}
		seen[id] = struct{}{}
		question := Question{ID: id, Position: len(questions), Title: normalizedQuestionTitle(item.Title, len(questions)), Required: value.Required, Supported: true}
		switch {
		case len(value.FileUploadQuestion) > 0:
			question.AnswerKind, question.Supported, question.UnsupportedCode = AnswerUnsupported, false, "file_upload"
		case len(value.ChoiceQuestion) > 0:
			var choice struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(value.ChoiceQuestion, &choice) != nil {
				return Form{}, &ProviderError{Code: "invalid_form_schema"}
			}
			if choice.Type == "CHECKBOX" {
				question.AnswerKind, question.Supported, question.UnsupportedCode = AnswerUnsupported, false, "multiple_answers"
			} else {
				question.AnswerKind = AnswerText
			}
		case len(value.TextQuestion) > 0:
			question.AnswerKind = AnswerText
		case len(value.ScaleQuestion) > 0:
			question.AnswerKind = AnswerScale
		case len(value.DateQuestion) > 0:
			question.AnswerKind = AnswerDate
		case len(value.TimeQuestion) > 0:
			question.AnswerKind = AnswerTime
		default:
			question.AnswerKind, question.Supported, question.UnsupportedCode = AnswerUnsupported, false, "unknown_question"
		}
		question.QuestionFingerprint = fingerprintQuestion(question)
		questions = append(questions, question)
	}
	if len(questions) == 0 {
		return Form{}, &ProviderError{Code: "empty_form"}
	}
	form := Form{ID: payload.FormID, Title: strings.TrimSpace(payload.Info.Title), Revision: strings.TrimSpace(payload.RevisionID), Questions: questions}
	form.Fingerprint = fingerprintForm(questions)
	return form, nil
}

func normalizeProviderResponses(payload providerResponses) (ResponsePage, error) {
	if len(payload.Responses) > 5000 || len(payload.NextPageToken) > 2048 {
		return ResponsePage{}, &ProviderError{Code: "invalid_response_page"}
	}
	responses := make([]Response, 0, len(payload.Responses))
	seen := make(map[string]struct{}, len(payload.Responses))
	for _, item := range payload.Responses {
		id := strings.TrimSpace(item.ResponseID)
		if id == "" || len(id) > 500 {
			return ResponsePage{}, &ProviderError{Code: "invalid_response"}
		}
		if _, duplicate := seen[id]; duplicate {
			return ResponsePage{}, &ProviderError{Code: "duplicate_response"}
		}
		seen[id] = struct{}{}
		timestamp := item.LastSubmittedTime
		if timestamp == "" {
			timestamp = item.CreateTime
		}
		submittedAt, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return ResponsePage{}, &ProviderError{Code: "invalid_response_time"}
		}
		answers := make(map[string][]string, len(item.Answers))
		for questionID, answer := range item.Answers {
			questionID = strings.TrimSpace(questionID)
			if questionID == "" || len(questionID) > 300 || answer.TextAnswers == nil {
				continue
			}
			values := make([]string, 0, len(answer.TextAnswers.Answers))
			for _, text := range answer.TextAnswers.Answers {
				if len(text.Value) > operations.MaximumCellBytes {
					return ResponsePage{}, &ProviderError{Code: "answer_too_large"}
				}
				values = append(values, text.Value)
			}
			answers[questionID] = values
		}
		responses = append(responses, Response{ID: id, SubmittedAt: submittedAt.UTC(), Answers: answers})
	}
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].SubmittedAt.Equal(responses[j].SubmittedAt) {
			return responses[i].ID < responses[j].ID
		}
		return responses[i].SubmittedAt.Before(responses[j].SubmittedAt)
	})
	return ResponsePage{Responses: responses, NextPageToken: payload.NextPageToken}, nil
}

func fingerprintQuestion(question Question) [sha256.Size]byte {
	value := fmt.Sprintf("%s\x00%s\x00%t\x00%t\x00%s", question.ID, question.AnswerKind, question.Required, question.Supported, question.UnsupportedCode)
	return sha256.Sum256([]byte(value))
}

func fingerprintForm(questions []Question) [sha256.Size]byte {
	parts := make([]string, 0, len(questions))
	for _, question := range questions {
		parts = append(parts, question.ID+":"+base64.RawURLEncoding.EncodeToString(question.QuestionFingerprint[:]))
	}
	sort.Strings(parts)
	return sha256.Sum256([]byte(strings.Join(parts, "\n")))
}

func normalizedQuestionTitle(title string, position int) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Sprintf("Pergunta %d", position+1)
	}
	if len(title) > 500 {
		return title[:500]
	}
	return title
}

func validProviderFormID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 10 || len(value) > 200 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func codeChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func decodeProviderJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
