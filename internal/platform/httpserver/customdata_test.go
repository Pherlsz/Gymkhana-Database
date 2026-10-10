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
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

type fakeCustomDataService struct {
	field       customdata.FieldDefinition
	fieldPage   customdata.FieldDefinitionPage
	options     []customdata.Option
	valueSet    customdata.ValueSet
	valueInputs []customdata.ValueInput
	target      customdata.TargetReference
	version     int64
	err         error
}

func (s *fakeCustomDataService) ListFieldDefinitions(context.Context, auth.Session, customdata.FieldDefinitionListOptions) (customdata.FieldDefinitionPage, error) {
	return s.fieldPage, s.err
}
func (s *fakeCustomDataService) ListOptions(context.Context, auth.Session, customdata.Identifier) ([]customdata.Option, error) {
	return s.options, s.err
}
func (s *fakeCustomDataService) GetValues(_ context.Context, _ auth.Session, target customdata.TargetReference) (customdata.ValueSet, error) {
	s.target = target
	return s.valueSet, s.err
}
func (s *fakeCustomDataService) ReplaceValues(_ context.Context, _ auth.Session, target customdata.TargetReference, version int64, values []customdata.ValueInput, _ string) (customdata.ValueSet, error) {
	s.target, s.version, s.valueInputs = target, version, values
	return s.valueSet, s.err
}

func customDataFixture(t *testing.T) (*fakeAdministrationService, *fakeCustomDataService, customdata.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	id, _ := customdata.NewIdentifier()
	now := time.Now().UTC()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{ID: actorID, Email: "admin", Role: auth.RoleAdmin, Active: true}}}}
	service := &fakeCustomDataService{
		field:    customdata.FieldDefinition{ID: id, Values: customdata.FieldDefinitionValues{TargetKind: customdata.TargetProfile, TechnicalKey: "shirt_size", Label: "Camiseta", Kind: customdata.FieldText, Active: true}, Version: 1, CreatedAt: now, UpdatedAt: now},
		valueSet: customdata.ValueSet{Target: customdata.TargetReference{Kind: customdata.ValueTargetProfile, ID: id}, Values: []customdata.StoredValue{}, Version: 2},
	}
	service.fieldPage = customdata.FieldDefinitionPage{Definitions: []customdata.FieldDefinition{service.field}, Total: 1, Limit: 100, SortField: customdata.FieldDefinitionSortLabel, SortOrder: customdata.SortAscending}
	return authentication, service, id, New(authTestLogger(), nil, Options{Auth: authentication, CustomData: service})
}

func customRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestCustomDataDefinitionsAndValuesRoutes(t *testing.T) {
	_, service, id, handler := customDataFixture(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, customRequest(http.MethodGet, "/api/v1/custom-fields?target_kind=PROFILE", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	body := `{"version":1,"values":[{"field_definition_id":"` + id.String() + `","field_kind":"TEXT","text":"G"}]}`
	handler.ServeHTTP(response, customRequest(http.MethodPut, "/api/v1/custom-values/profile/"+id.String(), body))
	if response.Code != http.StatusOK || service.version != 1 || service.target.Kind != customdata.ValueTargetProfile || len(service.valueInputs) != 1 || service.valueInputs[0].Text != "G" {
		t.Fatalf("replace status = %d, target = %#v, values = %#v, body = %s", response.Code, service.target, service.valueInputs, response.Body.String())
	}
}

func TestCustomDataMapsValidationAndAuthentication(t *testing.T) {
	authentication, service, _, handler := customDataFixture(t)
	service.err = &customdata.ValidationError{Fields: []customdata.FieldError{{Field: "label", Code: "required"}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, customRequest(http.MethodGet, "/api/v1/custom-fields?target_kind=PROFILE", ""))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.FieldErrors) != 1 || payload.FieldErrors[0].Field != "label" {
		t.Fatalf("payload = %#v, err = %v", payload, err)
	}

	authentication.sessionErr = auth.ErrUnauthenticated
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/custom-fields?target_kind=PROFILE", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("auth status = %d", response.Code)
	}
}
