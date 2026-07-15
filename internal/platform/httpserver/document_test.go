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
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeDocumentService struct {
	typePage       document.TypePage
	documentPage   document.Page
	typeValue      document.TypeDefinition
	documentValue  document.Document
	currentUse     document.CurrentUse
	typeList       document.TypeListOptions
	documentList   document.ListOptions
	typeValues     document.TypeValues
	documentValues document.Values
	version        int64
	confirmation   string
	holder         profile.Identifier
	assigned       bool
	returned       bool
	err            error
}

func (service *fakeDocumentService) ListTypes(_ context.Context, _ auth.Session, options document.TypeListOptions) (document.TypePage, error) {
	service.typeList = options
	return service.typePage, service.err
}
func (service *fakeDocumentService) GetType(context.Context, auth.Session, document.Identifier) (document.TypeDefinition, error) {
	return service.typeValue, service.err
}
func (service *fakeDocumentService) CreateType(_ context.Context, _ auth.Session, values document.TypeValues, _ string) (document.TypeDefinition, error) {
	service.typeValues = values
	return service.typeValue, service.err
}
func (service *fakeDocumentService) UpdateType(_ context.Context, _ auth.Session, _ document.Identifier, version int64, values document.TypeValues, _ string) (document.TypeDefinition, error) {
	service.version = version
	service.typeValues = values
	return service.typeValue, service.err
}
func (service *fakeDocumentService) DeleteType(_ context.Context, _ auth.Session, _ document.Identifier, version int64, confirmation, _ string) error {
	service.version = version
	service.confirmation = confirmation
	return service.err
}
func (service *fakeDocumentService) List(_ context.Context, _ auth.Session, options document.ListOptions) (document.Page, error) {
	service.documentList = options
	return service.documentPage, service.err
}
func (service *fakeDocumentService) Get(context.Context, auth.Session, document.Identifier) (document.Document, error) {
	return service.documentValue, service.err
}
func (service *fakeDocumentService) Create(_ context.Context, _ auth.Session, values document.Values, _ string) (document.Document, error) {
	service.documentValues = values
	return service.documentValue, service.err
}
func (service *fakeDocumentService) Update(_ context.Context, _ auth.Session, _ document.Identifier, version int64, values document.Values, _ string) (document.Document, error) {
	service.version = version
	service.documentValues = values
	return service.documentValue, service.err
}
func (service *fakeDocumentService) Duplicate(context.Context, auth.Session, document.Identifier, string) (document.Document, error) {
	return service.documentValue, service.err
}
func (service *fakeDocumentService) Delete(_ context.Context, _ auth.Session, _ document.Identifier, version int64, confirmation, _ string) error {
	service.version = version
	service.confirmation = confirmation
	return service.err
}
func (service *fakeDocumentService) AssignCurrentUse(_ context.Context, _ auth.Session, _ document.Identifier, holder profile.Identifier, _ string) (document.CurrentUse, error) {
	service.holder = holder
	service.assigned = true
	return service.currentUse, service.err
}
func (service *fakeDocumentService) ReturnCurrentUse(context.Context, auth.Session, document.Identifier, string) error {
	service.returned = true
	return service.err
}

func documentHTTPFixture(t *testing.T) (*fakeAdministrationService, *fakeDocumentService, document.Identifier, document.Identifier, profile.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	documentID, _ := document.NewIdentifier()
	typeID, _ := document.NewIdentifier()
	ownerID, _ := profile.NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	now := time.Now().UTC()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Login: "member", Role: auth.RoleMember, Active: true,
	}}}}
	typeValue := document.TypeDefinition{ID: typeID, Values: document.TypeValues{TechnicalKey: "rg", Label: "RG", Active: true, UniquenessPolicy: document.UniquenessPerProfile}, Version: 1, CreatedAt: now, UpdatedAt: now}
	currentUse := document.CurrentUse{HolderProfileID: holderID, AssignedAt: now}
	documentValue := document.Document{ID: documentID, Values: document.Values{OwnerProfileID: ownerID, TypeID: typeID, Identifier: "00AB-009", DocumentDate: "2026-07-15", RecordState: document.RecordCurrent}, Type: typeValue, Status: document.StatusInUse, CurrentUse: &currentUse, Version: 1, CreatedAt: now, UpdatedAt: now}
	service := &fakeDocumentService{
		typeValue:     typeValue,
		documentValue: documentValue,
		currentUse:    currentUse,
		typePage:      document.TypePage{Types: []document.TypeDefinition{typeValue}, Total: 1, Limit: 100, SortField: document.TypeSortLabel, SortOrder: document.SortAscending},
		documentPage:  document.Page{Documents: []document.Document{documentValue}, Total: 1, Limit: 100, SortField: document.SortIdentifier, SortOrder: document.SortAscending},
	}
	return authentication, service, documentID, typeID, ownerID, New(authTestLogger(), nil, Options{Auth: authentication, Document: service})
}

func TestDocumentRoutesListCreateAndCurrentUse(t *testing.T) {
	_, service, documentID, typeID, ownerID, handler := documentHTTPFixture(t)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/documents?limit=250&offset=10&sort=updated_at&order=desc&owner_profile_id="+ownerID.String()+"&document_type_id="+typeID.String()+"&identifier=00AB&status=IN_USE", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.documentList.Limit != 250 || service.documentList.Offset != 10 || service.documentList.SortField != document.SortUpdatedAt || service.documentList.Filters.Identifier != "00AB" {
		t.Fatalf("list status = %d, options = %#v, body = %s", listResponse.Code, service.documentList, listResponse.Body.String())
	}

	createBody := `{"owner_profile_id":"` + ownerID.String() + `","document_type_id":"` + typeID.String() + `","identifier_value":"00AB-009","document_date":"2026-07-15","notes":"","record_state":"CURRENT"}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || service.documentValues.Identifier != "00AB-009" || service.documentValues.OwnerProfileID != ownerID {
		t.Fatalf("create status = %d, values = %#v, body = %s", createResponse.Code, service.documentValues, createResponse.Body.String())
	}

	holderID := service.currentUse.HolderProfileID
	assignRequest := httptest.NewRequest(http.MethodPut, "/api/v1/documents/"+documentID.String()+"/current-use", strings.NewReader(`{"holder_profile_id":"`+holderID.String()+`"}`))
	assignRequest.Header.Set("Content-Type", "application/json")
	assignRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	assignResponse := httptest.NewRecorder()
	handler.ServeHTTP(assignResponse, assignRequest)
	if assignResponse.Code != http.StatusOK || !service.assigned || service.holder != holderID {
		t.Fatalf("assign status = %d, holder = %s, body = %s", assignResponse.Code, service.holder.String(), assignResponse.Body.String())
	}

	returnRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+documentID.String()+"/current-use", nil)
	returnRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	returnResponse := httptest.NewRecorder()
	handler.ServeHTTP(returnResponse, returnRequest)
	if returnResponse.Code != http.StatusNoContent || !service.returned {
		t.Fatalf("return status = %d, body = %s", returnResponse.Code, returnResponse.Body.String())
	}
}

func TestDocumentTypeRoutesAndPermanentDelete(t *testing.T) {
	_, service, _, typeID, _, handler := documentHTTPFixture(t)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/document-types?limit=500&sort=updated_at&order=desc&label=RG&active=true", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.typeList.Limit != 500 || service.typeList.SortField != document.TypeSortUpdatedAt || service.typeList.Filters.Active == nil || !*service.typeList.Filters.Active {
		t.Fatalf("list type status = %d, options = %#v, body = %s", listResponse.Code, service.typeList, listResponse.Body.String())
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/document-types", strings.NewReader(`{"technical_key":"rg","label":"RG","active":true,"uniqueness_policy":"PER_PROFILE","validation_regex":"^[A-Z0-9-]+$","date_required":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || service.typeValues.TechnicalKey != "rg" || !service.typeValues.DateRequired {
		t.Fatalf("create type status = %d, values = %#v, body = %s", createResponse.Code, service.typeValues, createResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/document-types/"+typeID.String(), strings.NewReader(`{"version":1,"confirmation":"Confirmar"}`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.version != 1 || service.confirmation != document.DeleteConfirmation {
		t.Fatalf("delete type status = %d, version = %d, confirmation = %q, body = %s", deleteResponse.Code, service.version, service.confirmation, deleteResponse.Body.String())
	}
}

func TestDocumentRoutesRequireAuthenticationAndMapErrors(t *testing.T) {
	authentication, service, documentID, _, _, handler := documentHTTPFixture(t)
	authentication.sessionErr = auth.ErrUnauthenticated
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	authentication.sessionErr = nil
	service.err = &document.ValidationError{Fields: []document.FieldError{{Field: "identifier_value", Code: "required"}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader(`{"owner_profile_id":"invalid","document_type_id":"invalid","identifier_value":"","document_date":"","notes":"","record_state":"CURRENT"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid identifiers status = %d, body = %s", response.Code, response.Body.String())
	}

	service.err = document.ErrConflict
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/documents/"+documentID.String(), strings.NewReader(`{"version":1,"confirmation":"Confirmar"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", response.Code, response.Body.String())
	}

	service.err = &document.ValidationError{Fields: []document.FieldError{{Field: "label", Code: "required"}}}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/document-types", strings.NewReader(`{"technical_key":"rg","label":"","active":true,"uniqueness_policy":"NONE","validation_regex":"","date_required":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.FieldErrors) != 1 || payload.FieldErrors[0].Field != "label" {
		t.Fatalf("payload = %#v, error = %v", payload, err)
	}
}
