package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeAdministrationService struct {
	fakeAuthenticationService
	users         []auth.ManagedUser
	updated       auth.ManagedUser
	updateInput   auth.UpdateUserAccessParams
	updateActor   auth.Session
	listRequestID string
	listErr       error
	updateErr     error
}

func (service *fakeAdministrationService) ListUsers(_ context.Context, actor auth.Session, _, _ int32, requestID string) ([]auth.ManagedUser, error) {
	service.updateActor = actor
	service.listRequestID = requestID
	return service.users, service.listErr
}

func (service *fakeAdministrationService) UpdateUserAccess(_ context.Context, actor auth.Session, params auth.UpdateUserAccessParams, _ string) (auth.ManagedUser, error) {
	service.updateActor = actor
	service.updateInput = params
	return service.updated, service.updateErr
}

func TestAdministrationListsAndUpdatesUsers(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	targetID, _ := auth.NewIdentifier()
	service := &fakeAdministrationService{
		fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
			ID: actorID, Login: "owner", Role: auth.RoleSuperadmin, Active: true,
		}}},
		users: []auth.ManagedUser{{User: auth.User{
			ID: targetID, Login: "member", DisplayName: "Member", Role: auth.RoleMember, Active: true,
		}, Version: 1}},
		updated: auth.ManagedUser{User: auth.User{
			ID: targetID, Login: "member", DisplayName: "Member", Role: auth.RoleAdmin, Active: true,
		}, Version: 2},
	}
	handler := New(authTestLogger(), nil, Options{Auth: service})

	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var listed adminUsersResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil || len(listed.Users) != 1 {
		t.Fatalf("listed = %#v, error = %v", listed, err)
	}
	if service.listRequestID == "" {
		t.Fatal("list request ID was not propagated")
	}

	body := `{"role":"ADMIN","active":true,"version":1}`
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+targetID.String()+"/access", strings.NewReader(body))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	if service.updateInput.UserID != targetID || service.updateInput.Role != auth.RoleAdmin || service.updateInput.Version != 1 {
		t.Fatalf("update input = %#v", service.updateInput)
	}
}

func TestAdministrationRequiresSession(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/admin/users", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
