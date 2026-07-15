package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeProfileService struct {
	page         profile.Page
	value        profile.Profile
	listOptions  profile.ListOptions
	createValues profile.Values
	updateValues profile.Values
	version      int64
	confirmation string
	err          error
}

func (service *fakeProfileService) List(_ context.Context, _ auth.Session, options profile.ListOptions) (profile.Page, error) {
	service.listOptions = options
	return service.page, service.err
}
func (service *fakeProfileService) Get(context.Context, auth.Session, profile.Identifier) (profile.Profile, error) {
	return service.value, service.err
}
func (service *fakeProfileService) Create(_ context.Context, _ auth.Session, values profile.Values, _ string) (profile.Profile, error) {
	service.createValues = values
	return service.value, service.err
}
func (service *fakeProfileService) Update(_ context.Context, _ auth.Session, _ profile.Identifier, version int64, values profile.Values, _ string) (profile.Profile, error) {
	service.version = version
	service.updateValues = values
	return service.value, service.err
}
func (service *fakeProfileService) Duplicate(context.Context, auth.Session, profile.Identifier, string) (profile.Profile, error) {
	return service.value, service.err
}
func (service *fakeProfileService) Delete(_ context.Context, _ auth.Session, _ profile.Identifier, version int64, confirmation, _ string) error {
	service.version = version
	service.confirmation = confirmation
	return service.err
}

func profileHTTPFixture(t *testing.T) (*fakeAdministrationService, *fakeProfileService, profile.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	id, _ := profile.NewIdentifier()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Login: "member", Role: auth.RoleMember, Active: true,
	}}}}
	value := profile.Profile{ID: id, Values: profile.Values{FullName: "Ana"}, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	service := &fakeProfileService{value: value, page: profile.Page{Profiles: []profile.Profile{value}, Total: 1, Limit: 100, SortField: profile.SortFullName, SortOrder: profile.SortAscending}}
	return authentication, service, id, New(authTestLogger(), nil, Options{Auth: authentication, Profile: service})
}

func TestProfileRoutesListCreateUpdateAndDelete(t *testing.T) {
	_, service, id, handler := profileHTTPFixture(t)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/profiles?limit=250&offset=10&sort=updated_at&order=desc&state=rs", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.listOptions.Limit != 250 || service.listOptions.Offset != 10 || service.listOptions.Filters.State != "rs" {
		t.Fatalf("list status = %d, options = %#v, body = %s", listResponse.Code, service.listOptions, listResponse.Body.String())
	}

	createBody := `{"full_name":"Ana","social_name":"","cpf":"","email":"","mobile_phone":"","landline_phone":"","address":{"street":"","number":"","complement":"","neighborhood":"","city":"","state":"","postal_code":""},"notes":""}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || service.createValues.FullName != "Ana" {
		t.Fatalf("create status = %d, values = %#v, body = %s", createResponse.Code, service.createValues, createResponse.Body.String())
	}

	updateBody := strings.TrimSuffix(createBody, "}") + `,"version":1}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+id.String(), strings.NewReader(updateBody))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK || service.version != 1 {
		t.Fatalf("update status = %d, version = %d, body = %s", updateResponse.Code, service.version, updateResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/profiles/"+id.String(), strings.NewReader(`{"version":1,"confirmation":"Confirmar"}`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.confirmation != profile.DeleteConfirmation {
		t.Fatalf("delete status = %d, confirmation = %q, body = %s", deleteResponse.Code, service.confirmation, deleteResponse.Body.String())
	}
}

func TestProfileRoutesReturnFieldErrors(t *testing.T) {
	_, service, _, handler := profileHTTPFixture(t)
	service.err = &profile.ValidationError{Fields: []profile.FieldError{{Field: "full_name", Code: "required"}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/profiles", strings.NewReader(`{"full_name":"","social_name":"","cpf":"","email":"","mobile_phone":"","landline_phone":"","address":{"street":"","number":"","complement":"","neighborhood":"","city":"","state":"","postal_code":""},"notes":""}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.FieldErrors) != 1 || payload.FieldErrors[0].Field != "full_name" {
		t.Fatalf("payload = %#v, error = %v", payload, err)
	}
}

func TestProfileRoutesRequireAuthenticationAndMapConflicts(t *testing.T) {
	authentication, service, id, handler := profileHTTPFixture(t)
	authentication.sessionErr = auth.ErrUnauthenticated
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	authentication.sessionErr = nil
	service.err = profile.ErrConflict
	request := httptest.NewRequest(http.MethodPut, "/api/v1/profiles/"+id.String(), strings.NewReader(`{"full_name":"Ana","social_name":"","cpf":"","email":"","mobile_phone":"","landline_phone":"","address":{"street":"","number":"","complement":"","neighborhood":"","city":"","state":"","postal_code":""},"notes":"","version":1}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", response.Code, response.Body.String())
	}
}
