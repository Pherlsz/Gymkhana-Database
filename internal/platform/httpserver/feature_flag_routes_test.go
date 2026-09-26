package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/featureflags"
)

type memoryFeatureFlags struct {
	flags map[string]bool
}

func (store *memoryFeatureFlags) List(_ context.Context, actor auth.Session) ([]featureflags.Flag, error) {
	if actor.User.Role != auth.RoleSuperadmin || !actor.User.Active {
		return nil, featureflags.ErrForbidden
	}
	out := make([]featureflags.Flag, 0, len(featureflags.Known))
	for _, key := range featureflags.Known {
		out = append(out, featureflags.Flag{Key: key, Enabled: store.flags[key]})
	}
	return out, nil
}

func (store *memoryFeatureFlags) SetEnabled(_ context.Context, actor auth.Session, key string, enabled bool) (featureflags.Flag, error) {
	if actor.User.Role != auth.RoleSuperadmin || !actor.User.Active {
		return featureflags.Flag{}, featureflags.ErrForbidden
	}
	if !featureflags.IsKnown(key) {
		return featureflags.Flag{}, featureflags.ErrUnknownFlag
	}
	if store.flags == nil {
		store.flags = map[string]bool{}
	}
	store.flags[key] = enabled
	return featureflags.Flag{Key: key, Enabled: enabled}, nil
}

func (store *memoryFeatureFlags) IsEnabled(_ context.Context, key string) (bool, error) {
	return store.flags[key], nil
}

func TestFeatureFlagsRequireSuperadmin(t *testing.T) {
	flags := &memoryFeatureFlags{flags: map[string]bool{featureflags.KeyAIChat: true}}
	admin := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		session: auth.Session{User: auth.User{Role: auth.RoleAdmin, Active: true}},
	}}
	handler := New(authTestLogger(), nil, Options{Auth: admin, FeatureFlagAdmin: flags})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/feature-flags", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}

	admin.session = auth.Session{User: auth.User{Role: auth.RoleSuperadmin, Active: true}}
	request = httptest.NewRequest(http.MethodGet, "/api/admin/feature-flags", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	listed, ok := body["flags"].([]any)
	if !ok || len(listed) != len(featureflags.Known) {
		t.Fatalf("flags = %#v", body["flags"])
	}
}
