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
	entityType       customdata.EntityType
	entityTypePage   customdata.EntityTypePage
	field            customdata.FieldDefinition
	fieldPage        customdata.FieldDefinitionPage
	options          []customdata.Option
	option           customdata.Option
	valueSet         customdata.ValueSet
	entity           customdata.Entity
	entityPage       customdata.EntityPage
	entityTypeValues customdata.EntityTypeValues
	fieldValues      customdata.FieldDefinitionValues
	optionValues     customdata.OptionValues
	valueInputs      []customdata.ValueInput
	target           customdata.TargetReference
	entityList       customdata.EntityListOptions
	version          int64
	confirmation     string
	err              error
}

func (s *fakeCustomDataService) ListEntityTypes(context.Context, auth.Session, customdata.EntityTypeListOptions) (customdata.EntityTypePage, error) {
	return s.entityTypePage, s.err
}
func (s *fakeCustomDataService) GetEntityType(context.Context, auth.Session, customdata.Identifier) (customdata.EntityType, error) {
	return s.entityType, s.err
}
func (s *fakeCustomDataService) CreateEntityType(_ context.Context, _ auth.Session, v customdata.EntityTypeValues, _ string) (customdata.EntityType, error) {
	s.entityTypeValues = v
	return s.entityType, s.err
}
func (s *fakeCustomDataService) UpdateEntityType(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, v customdata.EntityTypeValues, _ string) (customdata.EntityType, error) {
	s.version, s.entityTypeValues = version, v
	return s.entityType, s.err
}
func (s *fakeCustomDataService) DeleteEntityType(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, confirmation, _ string) error {
	s.version, s.confirmation = version, confirmation
	return s.err
}
func (s *fakeCustomDataService) ListFieldDefinitions(context.Context, auth.Session, customdata.FieldDefinitionListOptions) (customdata.FieldDefinitionPage, error) {
	return s.fieldPage, s.err
}
func (s *fakeCustomDataService) GetFieldDefinition(context.Context, auth.Session, customdata.Identifier) (customdata.FieldDefinition, error) {
	return s.field, s.err
}
func (s *fakeCustomDataService) CreateFieldDefinition(_ context.Context, _ auth.Session, v customdata.FieldDefinitionValues, _ string) (customdata.FieldDefinition, error) {
	s.fieldValues = v
	return s.field, s.err
}
func (s *fakeCustomDataService) UpdateFieldDefinition(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, v customdata.FieldDefinitionValues, _ string) (customdata.FieldDefinition, error) {
	s.version, s.fieldValues = version, v
	return s.field, s.err
}
func (s *fakeCustomDataService) DeleteFieldDefinition(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, confirmation, _ string) error {
	s.version, s.confirmation = version, confirmation
	return s.err
}
func (s *fakeCustomDataService) ListOptions(context.Context, auth.Session, customdata.Identifier) ([]customdata.Option, error) {
	return s.options, s.err
}
func (s *fakeCustomDataService) CreateOption(_ context.Context, _ auth.Session, _ customdata.Identifier, v customdata.OptionValues, _ string) (customdata.Option, error) {
	s.optionValues = v
	return s.option, s.err
}
func (s *fakeCustomDataService) UpdateOption(_ context.Context, _ auth.Session, _, _ customdata.Identifier, version int64, v customdata.OptionValues, _ string) (customdata.Option, error) {
	s.version, s.optionValues = version, v
	return s.option, s.err
}
func (s *fakeCustomDataService) DeleteOption(_ context.Context, _ auth.Session, _, _ customdata.Identifier, version int64, confirmation, _ string) error {
	s.version, s.confirmation = version, confirmation
	return s.err
}
func (s *fakeCustomDataService) GetValues(_ context.Context, _ auth.Session, target customdata.TargetReference) (customdata.ValueSet, error) {
	s.target = target
	return s.valueSet, s.err
}
func (s *fakeCustomDataService) ReplaceValues(_ context.Context, _ auth.Session, target customdata.TargetReference, version int64, values []customdata.ValueInput, _ string) (customdata.ValueSet, error) {
	s.target, s.version, s.valueInputs = target, version, values
	return s.valueSet, s.err
}
func (s *fakeCustomDataService) ListEntities(_ context.Context, _ auth.Session, options customdata.EntityListOptions) (customdata.EntityPage, error) {
	s.entityList = options
	return s.entityPage, s.err
}
func (s *fakeCustomDataService) GetEntity(context.Context, auth.Session, customdata.Identifier) (customdata.Entity, error) {
	return s.entity, s.err
}
func (s *fakeCustomDataService) CreateEntity(_ context.Context, _ auth.Session, _ customdata.Identifier, _ *customdata.Identifier, values []customdata.ValueInput, _ string) (customdata.Entity, error) {
	s.valueInputs = values
	return s.entity, s.err
}
func (s *fakeCustomDataService) UpdateEntity(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, values []customdata.ValueInput, _ string) (customdata.Entity, error) {
	s.version, s.valueInputs = version, values
	return s.entity, s.err
}
func (s *fakeCustomDataService) DeleteEntity(_ context.Context, _ auth.Session, _ customdata.Identifier, version int64, confirmation, _ string) error {
	s.version, s.confirmation = version, confirmation
	return s.err
}

func customDataFixture(t *testing.T) (*fakeAdministrationService, *fakeCustomDataService, customdata.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	id, _ := customdata.NewIdentifier()
	now := time.Now().UTC()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{ID: actorID, Email: "admin", Role: auth.RoleAdmin, Active: true}}}}
	service := &fakeCustomDataService{
		entityType: customdata.EntityType{ID: id, Values: customdata.EntityTypeValues{TechnicalKey: "vehicle", Label: "Veículo", Active: true, ProfileCardinality: customdata.CardinalityManyPerProfile}, Version: 1, CreatedAt: now, UpdatedAt: now},
		field:      customdata.FieldDefinition{ID: id, Values: customdata.FieldDefinitionValues{TargetKind: customdata.TargetProfile, TechnicalKey: "shirt_size", Label: "Camiseta", Kind: customdata.FieldText, Active: true}, Version: 1, CreatedAt: now, UpdatedAt: now},
		valueSet:   customdata.ValueSet{Target: customdata.TargetReference{Kind: customdata.ValueTargetProfile, ID: id}, Values: []customdata.StoredValue{}, Version: 2},
		entity:     customdata.Entity{ID: id, TypeID: id, Values: []customdata.StoredValue{}, Version: 1, CreatedAt: now, UpdatedAt: now},
	}
	service.entityTypePage = customdata.EntityTypePage{Types: []customdata.EntityType{service.entityType}, Total: 1, Limit: 100, SortField: customdata.EntityTypeSortLabel, SortOrder: customdata.SortAscending}
	service.fieldPage = customdata.FieldDefinitionPage{Definitions: []customdata.FieldDefinition{service.field}, Total: 1, Limit: 100, SortField: customdata.FieldDefinitionSortLabel, SortOrder: customdata.SortAscending}
	service.entityPage = customdata.EntityPage{Entities: []customdata.Entity{service.entity}, Total: 1, Limit: 100}
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
	handler.ServeHTTP(response, customRequest(http.MethodGet, "/api/v1/custom-entity-types", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, customRequest(http.MethodPost, "/api/v1/custom-fields", `{"target_kind":"PROFILE","technical_key":"shirt_size","label":"Camiseta","field_kind":"TEXT","required":false,"active":true}`))
	if response.Code != http.StatusCreated || service.fieldValues.TechnicalKey != "shirt_size" {
		t.Fatalf("create status = %d, values = %#v, body = %s", response.Code, service.fieldValues, response.Body.String())
	}

	response = httptest.NewRecorder()
	body := `{"version":1,"values":[{"field_definition_id":"` + id.String() + `","field_kind":"TEXT","text":"G"}]}`
	handler.ServeHTTP(response, customRequest(http.MethodPut, "/api/v1/custom-values/profile/"+id.String(), body))
	if response.Code != http.StatusOK || service.version != 1 || service.target.Kind != customdata.ValueTargetProfile || len(service.valueInputs) != 1 || service.valueInputs[0].Text != "G" {
		t.Fatalf("replace status = %d, target = %#v, values = %#v, body = %s", response.Code, service.target, service.valueInputs, response.Body.String())
	}
}

func TestCustomEntityFiltersAndDelete(t *testing.T) {
	_, service, id, handler := customDataFixture(t)
	response := httptest.NewRecorder()
	path := "/api/v1/custom-entities?entity_type_id=" + id.String() + "&owner_profile_id=" + id.String() + "&limit=250&offset=10"
	handler.ServeHTTP(response, customRequest(http.MethodGet, path, ""))
	if response.Code != http.StatusOK || service.entityList.Limit != 250 || service.entityList.Offset != 10 || service.entityList.OwnerProfileID == nil {
		t.Fatalf("list status = %d, options = %#v, body = %s", response.Code, service.entityList, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, customRequest(http.MethodDelete, "/api/v1/custom-entities/"+id.String(), `{"version":3,"confirmation":"Confirmar"}`))
	if response.Code != http.StatusNoContent || service.version != 3 || service.confirmation != customdata.DeleteConfirmation {
		t.Fatalf("delete status = %d, version = %d, confirmation = %q", response.Code, service.version, service.confirmation)
	}
}

func TestCustomDataMapsValidationAndAuthentication(t *testing.T) {
	authentication, service, _, handler := customDataFixture(t)
	service.err = &customdata.ValidationError{Fields: []customdata.FieldError{{Field: "label", Code: "required"}}}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, customRequest(http.MethodPost, "/api/v1/custom-entity-types", `{"technical_key":"vehicle","label":"","active":true,"profile_cardinality":"MANY_PER_PROFILE"}`))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.FieldErrors) != 1 || payload.FieldErrors[0].Field != "label" {
		t.Fatalf("payload = %#v, err = %v", payload, err)
	}

	authentication.sessionErr = auth.ErrUnauthenticated
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/custom-entity-types", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("auth status = %d", response.Code)
	}
}
