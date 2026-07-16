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
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeBillService struct {
	typePage     bill.TypePage
	billPage     bill.Page
	typeValue    bill.TypeDefinition
	billValue    bill.Bill
	currentUse   bill.CurrentUse
	typeList     bill.TypeListOptions
	billList     bill.ListOptions
	typeValues   bill.TypeValues
	billValues   bill.Values
	version      int64
	confirmation string
	holder       profile.Identifier
	assigned     bool
	returned     bool
	err          error
}

func (service *fakeBillService) ListTypes(_ context.Context, _ auth.Session, options bill.TypeListOptions) (bill.TypePage, error) {
	service.typeList = options
	return service.typePage, service.err
}
func (service *fakeBillService) GetType(context.Context, auth.Session, bill.Identifier) (bill.TypeDefinition, error) {
	return service.typeValue, service.err
}
func (service *fakeBillService) CreateType(_ context.Context, _ auth.Session, values bill.TypeValues, _ string) (bill.TypeDefinition, error) {
	service.typeValues = values
	return service.typeValue, service.err
}
func (service *fakeBillService) UpdateType(_ context.Context, _ auth.Session, _ bill.Identifier, version int64, values bill.TypeValues, _ string) (bill.TypeDefinition, error) {
	service.version = version
	service.typeValues = values
	return service.typeValue, service.err
}
func (service *fakeBillService) DeleteType(_ context.Context, _ auth.Session, _ bill.Identifier, version int64, confirmation, _ string) error {
	service.version = version
	service.confirmation = confirmation
	return service.err
}
func (service *fakeBillService) List(_ context.Context, _ auth.Session, options bill.ListOptions) (bill.Page, error) {
	service.billList = options
	return service.billPage, service.err
}
func (service *fakeBillService) Get(context.Context, auth.Session, bill.Identifier) (bill.Bill, error) {
	return service.billValue, service.err
}
func (service *fakeBillService) Create(_ context.Context, _ auth.Session, values bill.Values, _ string) (bill.Bill, error) {
	service.billValues = values
	return service.billValue, service.err
}
func (service *fakeBillService) Update(_ context.Context, _ auth.Session, _ bill.Identifier, version int64, values bill.Values, _ string) (bill.Bill, error) {
	service.version = version
	service.billValues = values
	return service.billValue, service.err
}
func (service *fakeBillService) Duplicate(context.Context, auth.Session, bill.Identifier, string) (bill.Bill, error) {
	return service.billValue, service.err
}
func (service *fakeBillService) Delete(_ context.Context, _ auth.Session, _ bill.Identifier, version int64, confirmation, _ string) error {
	service.version = version
	service.confirmation = confirmation
	return service.err
}
func (service *fakeBillService) AssignCurrentUse(_ context.Context, _ auth.Session, _ bill.Identifier, holder profile.Identifier, _ string) (bill.CurrentUse, error) {
	service.holder = holder
	service.assigned = true
	return service.currentUse, service.err
}
func (service *fakeBillService) ReturnCurrentUse(context.Context, auth.Session, bill.Identifier, string) error {
	service.returned = true
	return service.err
}

func billHTTPFixture(t *testing.T) (*fakeAdministrationService, *fakeBillService, bill.Identifier, bill.Identifier, profile.Identifier, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	billID, _ := bill.NewIdentifier()
	typeID, _ := bill.NewIdentifier()
	ownerID, _ := profile.NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	now := time.Now().UTC()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Login: "member", Role: auth.RoleMember, Active: true,
	}}}}
	typeValue := bill.TypeDefinition{ID: typeID, Values: bill.TypeValues{TechnicalKey: "rg", Label: "RG", Active: true, SupportsCurrentUse: true}, Version: 1, CreatedAt: now, UpdatedAt: now}
	currentUse := bill.CurrentUse{HolderProfileID: holderID, AssignedAt: now}
	billValue := bill.Bill{ID: billID, Values: bill.Values{OwnerProfileID: ownerID, TypeID: typeID, PrintedHolderName: "Ana", PrintedAddress: "Rua A, 10", Reference: "UC-009", Competence: "2026-07", Amount: "123.45", Currency: "BRL", RecordState: bill.RecordCurrent}, Type: typeValue, Status: bill.StatusInUse, CurrentUse: &currentUse, Version: 1, CreatedAt: now, UpdatedAt: now}
	service := &fakeBillService{
		typeValue:  typeValue,
		billValue:  billValue,
		currentUse: currentUse,
		typePage:   bill.TypePage{Types: []bill.TypeDefinition{typeValue}, Total: 1, Limit: 100, SortField: bill.TypeSortLabel, SortOrder: bill.SortAscending},
		billPage:   bill.Page{Bills: []bill.Bill{billValue}, Total: 1, Limit: 100, SortField: bill.SortReference, SortOrder: bill.SortAscending},
	}
	return authentication, service, billID, typeID, ownerID, New(authTestLogger(), nil, Options{Auth: authentication, Bill: service})
}

func TestBillRoutesListCreateAndCurrentUse(t *testing.T) {
	_, service, billID, typeID, ownerID, handler := billHTTPFixture(t)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/bills?limit=250&offset=10&sort=updated_at&order=desc&owner_profile_id="+ownerID.String()+"&bill_type_id="+typeID.String()+"&reference=UC&competence=2026-07&status=IN_USE", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.billList.Limit != 250 || service.billList.Offset != 10 || service.billList.SortField != bill.SortUpdatedAt || service.billList.Filters.Reference != "UC" || service.billList.Filters.Competence != "2026-07" {
		t.Fatalf("list status = %d, options = %#v, body = %s", listResponse.Code, service.billList, listResponse.Body.String())
	}

	createBody := `{"owner_profile_id":"` + ownerID.String() + `","bill_type_id":"` + typeID.String() + `","printed_holder_name":"Ana","printed_address":"Rua A, 10","reference_value":"UC-009","competence":"2026-07","amount":"123.45","currency":"BRL","notes":"","record_state":"CURRENT"}`
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/bills", strings.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || service.billValues.Reference != "UC-009" || service.billValues.Amount != "123.45" || service.billValues.OwnerProfileID != ownerID {
		t.Fatalf("create status = %d, values = %#v, body = %s", createResponse.Code, service.billValues, createResponse.Body.String())
	}

	holderID := service.currentUse.HolderProfileID
	assignRequest := httptest.NewRequest(http.MethodPut, "/api/v1/bills/"+billID.String()+"/current-use", strings.NewReader(`{"holder_profile_id":"`+holderID.String()+`"}`))
	assignRequest.Header.Set("Content-Type", "application/json")
	assignRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	assignResponse := httptest.NewRecorder()
	handler.ServeHTTP(assignResponse, assignRequest)
	if assignResponse.Code != http.StatusOK || !service.assigned || service.holder != holderID {
		t.Fatalf("assign status = %d, holder = %s, body = %s", assignResponse.Code, service.holder.String(), assignResponse.Body.String())
	}

	returnRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/bills/"+billID.String()+"/current-use", nil)
	returnRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	returnResponse := httptest.NewRecorder()
	handler.ServeHTTP(returnResponse, returnRequest)
	if returnResponse.Code != http.StatusNoContent || !service.returned {
		t.Fatalf("return status = %d, body = %s", returnResponse.Code, returnResponse.Body.String())
	}
}

func TestBillTypeRoutesAndPermanentDelete(t *testing.T) {
	_, service, _, typeID, _, handler := billHTTPFixture(t)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/bill-types?limit=500&sort=updated_at&order=desc&label=RG&active=true", nil)
	listRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || service.typeList.Limit != 500 || service.typeList.SortField != bill.TypeSortUpdatedAt || service.typeList.Filters.Active == nil || !*service.typeList.Filters.Active {
		t.Fatalf("list type status = %d, options = %#v, body = %s", listResponse.Code, service.typeList, listResponse.Body.String())
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/bill-types", strings.NewReader(`{"technical_key":"energia","label":"Energia","active":true,"supports_current_use":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated || service.typeValues.TechnicalKey != "energia" || !service.typeValues.SupportsCurrentUse {
		t.Fatalf("create type status = %d, values = %#v, body = %s", createResponse.Code, service.typeValues, createResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/bill-types/"+typeID.String(), strings.NewReader(`{"version":1,"confirmation":"Confirmar"}`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.version != 1 || service.confirmation != bill.DeleteConfirmation {
		t.Fatalf("delete type status = %d, version = %d, confirmation = %q, body = %s", deleteResponse.Code, service.version, service.confirmation, deleteResponse.Body.String())
	}
}

func TestBillRoutesRequireAuthenticationAndMapErrors(t *testing.T) {
	authentication, service, billID, _, _, handler := billHTTPFixture(t)
	authentication.sessionErr = auth.ErrUnauthenticated
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	authentication.sessionErr = nil
	service.err = &bill.ValidationError{Fields: []bill.FieldError{{Field: "amount", Code: "invalid_format"}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/bills", strings.NewReader(`{"owner_profile_id":"invalid","bill_type_id":"invalid","printed_holder_name":"","printed_address":"","reference_value":"","competence":"","amount":"","currency":"","notes":"","record_state":"CURRENT"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid identifiers status = %d, body = %s", response.Code, response.Body.String())
	}

	service.err = bill.ErrConflict
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/bills/"+billID.String(), strings.NewReader(`{"version":1,"confirmation":"Confirmar"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, body = %s", response.Code, response.Body.String())
	}

	service.err = &bill.ValidationError{Fields: []bill.FieldError{{Field: "label", Code: "required"}}}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/bill-types", strings.NewReader(`{"technical_key":"energia","label":"","active":true,"supports_current_use":true}`))
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
