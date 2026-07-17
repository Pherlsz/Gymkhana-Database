package googleforms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGoogleProviderOAuthAndFormsFlow(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") == "authorization_code" {
				if r.Form.Get("code_verifier") != "verifier" {
					t.Fatalf("code_verifier = %q", r.Form.Get("code_verifier"))
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "refresh_token": "refresh", "scope": strings.Join(RequiredScopes, " ")})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access"})
		case "/v1/forms/form_identifier_123":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"formId": "form_identifier_123", "revisionId": "revision-1", "info": map[string]any{"title": "Inscrições"},
				"items": []any{
					map[string]any{"itemId": "item1", "title": "Nome", "questionItem": map[string]any{"question": map[string]any{"questionId": "question-name", "required": true, "textQuestion": map[string]any{}}}},
					map[string]any{"itemId": "item2", "title": "Opções", "questionItem": map[string]any{"question": map[string]any{"questionId": "question-check", "choiceQuestion": map[string]any{"type": "CHECKBOX"}}}},
				},
			})
		case "/v1/forms/form_identifier_123/responses":
			if got := r.URL.Query().Get("filter"); !strings.HasPrefix(got, "timestamp >= 2026-07-17T") {
				t.Fatalf("filter = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"responses": []any{map[string]any{
					"responseId": "response-1", "lastSubmittedTime": "2026-07-17T12:00:00Z",
					"answers": map[string]any{"question-name": map[string]any{"textAnswers": map[string]any{"answers": []any{map[string]any{"value": "Ana"}}}}},
				}},
			})
		case "/revoke":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "http://app.test/callback", HTTPClient: server.Client(),
		AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", RevokeURL: server.URL + "/revoke", FormsBaseURL: server.URL + "/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, err := provider.AuthorizationURL("state", codeChallenge("verifier"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(authorizationURL)
	if parsed.Query().Get("scope") != strings.Join(RequiredScopes, " ") || parsed.Query().Get("access_type") != "offline" || parsed.Query().Get("state") != "state" {
		t.Fatalf("authorization query = %v", parsed.Query())
	}
	token, err := provider.Exchange(context.Background(), "code", "verifier")
	if err != nil || token.RefreshToken != "refresh" || !hasRequiredScopes(token.Scopes) {
		t.Fatalf("Exchange() = %#v, %v", token, err)
	}
	form, err := provider.GetForm(context.Background(), token.RefreshToken, "form_identifier_123")
	if err != nil || form.Title != "Inscrições" || len(form.Questions) != 2 || form.Questions[1].UnsupportedCode != "multiple_answers" {
		t.Fatalf("GetForm() = %#v, %v", form, err)
	}
	after := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)
	page, err := provider.ListResponses(context.Background(), token.RefreshToken, form.ID, &after, "", 100)
	if err != nil || len(page.Responses) != 1 || page.Responses[0].Answers["question-name"][0] != "Ana" {
		t.Fatalf("ListResponses() = %#v, %v", page, err)
	}
	if err := provider.Revoke(context.Background(), token.RefreshToken); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
}

func TestGoogleProviderClassifiesRetryAndReauthorization(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		want      error
		retryable bool
	}{
		{name: "invalid grant", status: http.StatusBadRequest, body: `{"error":"invalid_grant"}`, want: ErrNeedsReauth},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{}`, want: ErrRateLimited, retryable: true},
		{name: "unavailable", status: http.StatusServiceUnavailable, body: `{}`, want: ErrProviderRetryable, retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			provider, err := NewGoogleProvider(GoogleProviderOptions{ClientID: "client", ClientSecret: "secret", RedirectURL: "http://app.test/callback", HTTPClient: server.Client(), TokenURL: server.URL, AuthorizeURL: server.URL, RevokeURL: server.URL, FormsBaseURL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Exchange(context.Background(), "code", "verifier")
			if !errors.Is(err, test.want) {
				t.Fatalf("Exchange() error = %v, want %v", err, test.want)
			}
			var providerError *ProviderError
			if !errors.As(err, &providerError) || providerError.Retryable != test.retryable {
				t.Fatalf("ProviderError = %#v", providerError)
			}
		})
	}
}

func TestFormFingerprintIgnoresTitleButDetectsTypeChange(t *testing.T) {
	base := providerForm{FormID: "form_identifier_123", RevisionID: "r1"}
	base.Info.Title = "Form"
	base.Items = []providerItem{{ItemID: "item", Title: "Original", QuestionItem: &providerQuestionItem{Question: providerQuestion{QuestionID: "question-1", TextQuestion: json.RawMessage(`{}`)}}}}
	first, err := normalizeProviderForm(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Items[0].Title = "Renamed"
	second, err := normalizeProviderForm(base)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatal("title-only change altered schema fingerprint")
	}
	base.Items[0].QuestionItem.Question.TextQuestion = nil
	base.Items[0].QuestionItem.Question.DateQuestion = json.RawMessage(`{}`)
	third, err := normalizeProviderForm(base)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == third.Fingerprint {
		t.Fatal("question type change did not alter schema fingerprint")
	}
}
