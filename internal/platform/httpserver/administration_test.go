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

type fakeUserAdministrationService struct {
	users       []auth.ManagedUser
	listActor   auth.User
	updateActor auth.User
	update      auth.UserAccessUpdate
	updated     auth.ManagedUser
	err         error
}

func (service *fakeUserAdministrationService) ListUsers(_ context.Context, actor auth.User, _, _ int32, _ string) ([]auth.ManagedUser, error) {
	service.listActor = actor
	return service.users, service.err
}

func (service *fakeUserAdministrationService) UpdateUserAccess(
	_ context.Context,
	actor auth.User,
	update auth.UserAccessUpdate,
	_ string,
) (auth.ManagedUser, error) {
	service.updateActor = actor
	service.update = update
	return service.updated, service.err
}

func TestAdministrationRoutesRequireAdministrativeRole(t *testing.T) {
	authentication := &fakeAuthenticationService{
		session: auth.Session{User: auth.User{Login: "member", Role: auth.RoleMember, Active: true}},
	}
	handler := New(authTestLogger(), nil, Options{
		Auth:  authentication,
		Users: &fakeUserAdministrationService{},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-value"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestAdministrationRoutesListAndUpdateUsers(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	memberID, _ := auth.NewIdentifier()
	superadminID, _ := auth.NewIdentifier()
	authentication := &fakeAuthenticationService{
		session: auth.Session{User: auth.User{ID: actorID, Login: "admin", Role: auth.RoleAdmin, Active: true}},
	}
	administration := &fakeUserAdministrationService{
		users: []auth.ManagedUser{
			{ID: memberID, Login: "member", DisplayName: "Member", Role: auth.RoleMember, Active: true, Version: 2},
			{ID: superadminID, Login: "owner", DisplayName: "Owner", Role: auth.RoleSuperadmin, Active: true, Version: 1},
		},
		updated: auth.ManagedUser{ID: memberID, Login: "member", DisplayName: "Member", Role: auth.RoleAdmin, Active: true, Version: 3},
	}
	handler := New(authTestLogger(), nil, Options{Auth: authentication, Users: administration})

	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-value"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d", listResponse.Code)
	}
	var listed managedUsersResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Users) != 2 || !listed.Users[1].Protected || administration.listActor.ID != actorID {
		t.Fatalf("listed = %#v, actor = %#v", listed, administration.listActor)
	}

	updateRequest := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/users/"+memberID.String()+"/access",
		strings.NewReader(`{"role":"ADMIN","active":true,"version":2}`),
	)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-value"})
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	if administration.update.UserID != memberID || administration.update.Role != auth.RoleAdmin || administration.updateActor.ID != actorID {
		t.Fatalf("update = %#v, actor = %#v", administration.update, administration.updateActor)
	}
}
