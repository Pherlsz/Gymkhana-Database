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
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

type fakeOperationsService struct {
	catalog    []operations.ModuleCatalog
	imported   operations.Import
	report     operations.Report
	exported   operations.Export
	bulk       operations.BulkDeleteResult
	err        error
	lastModule operations.Module
	lastItems  []operations.BulkItem
}

func (service *fakeOperationsService) GetReport(context.Context, auth.Session, operations.Identifier) (operations.Report, error) {
	return service.report, service.err
}

func (service *fakeOperationsService) Catalog(context.Context, auth.Session) ([]operations.ModuleCatalog, error) {
	return service.catalog, service.err
}

func (service *fakeOperationsService) CreateImport(context.Context, auth.Session, operations.CreateImportInput, string) (operations.UploadGrant, error) {
	return operations.UploadGrant{Import: service.imported, UploadURL: "https://storage.invalid/signed", Method: "PUT", Headers: map[string]string{"Content-Type": "application/octet-stream"}, ExpiresAt: service.imported.ExpiresAt}, service.err
}

func (service *fakeOperationsService) ConfirmImport(context.Context, auth.Session, operations.Identifier, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) GetImport(context.Context, auth.Session, operations.Identifier) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) ListImports(context.Context, auth.Session, operations.ListOptions) (operations.ImportPage, error) {
	return operations.ImportPage{Imports: []operations.Import{service.imported}, Total: 1, Limit: 50}, service.err
}

func (service *fakeOperationsService) SelectSheet(context.Context, auth.Session, operations.Identifier, int64, int, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) SaveMapping(context.Context, auth.Session, operations.Identifier, int64, []operations.MappingInput, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) Preview(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) SaveDecisions(context.Context, auth.Session, operations.Identifier, int64, []operations.DecisionInput, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) Execute(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) CancelImport(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error) {
	return service.imported, service.err
}

func (service *fakeOperationsService) CreateExport(_ context.Context, _ auth.Session, module operations.Module, _, _ string) (operations.Export, error) {
	service.lastModule = module
	return service.exported, service.err
}

func (service *fakeOperationsService) GetExport(context.Context, auth.Session, operations.Identifier) (operations.Export, error) {
	return service.exported, service.err
}

func (service *fakeOperationsService) ListExports(context.Context, auth.Session, operations.ListOptions) (operations.ExportPage, error) {
	return operations.ExportPage{Exports: []operations.Export{service.exported}, Total: 1, Limit: 50}, service.err
}

func (service *fakeOperationsService) DownloadExport(context.Context, auth.Session, operations.Identifier, string) (operations.DownloadGrant, error) {
	return operations.DownloadGrant{URL: "https://storage.invalid/download", Method: "GET", ExpiresAt: service.exported.ExpiresAt}, service.err
}

func (service *fakeOperationsService) BulkDelete(_ context.Context, _ auth.Session, module operations.Module, items []operations.BulkItem, _, _ string) (operations.BulkDeleteResult, error) {
	service.lastModule = module
	service.lastItems = items
	return service.bulk, service.err
}

func operationsHTTPFixture(t *testing.T, service *fakeOperationsService) http.Handler {
	t.Helper()
	actorID, err := auth.NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		session: auth.Session{User: auth.User{ID: actorID, Email: "admin", Role: auth.RoleAdmin, Active: true}},
	}}
	return New(authTestLogger(), nil, Options{Auth: authentication, Operations: service})
}

func TestOperationsHTTPUsesLogicalRedactedContracts(t *testing.T) {
	id, _ := operations.NewIdentifier()
	actorID, _ := auth.NewIdentifier()
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	service := &fakeOperationsService{
		catalog: []operations.ModuleCatalog{{
			ID: operations.ModuleProfiles, Label: "Pessoas", CanImport: true, CanExport: true,
			CanDuplicate: true, CanDelete: true, CanBulkDelete: true,
			Fields: []operations.Field{{ID: "full_name", Label: "Nome", Kind: operations.FieldText, Required: true, Importable: true, Exportable: true}},
		}},
		imported: operations.Import{
			ID: id, ActorUserID: actorID, Module: operations.ModuleProfiles, SourceKind: operations.SourceXLSX,
			OriginalFilename: "people.xlsx", DeclaredSize: 123, ActualSize: 123,
			ObjectKey: "operations/private/secret.xlsx", RiverJobID: 987, ContentSHA256: [32]byte{9},
			State: operations.ImportReady, Stage: operations.StagePreview, ExpiresAt: now.Add(time.Hour),
			Version: 3, CreatedAt: now, UpdatedAt: now,
			Preview: []operations.Row{{SheetIndex: 0, RowNumber: 2, ProposedAction: operations.ActionCreate,
				Cells: []operations.Cell{{SourceColumn: 0, RawValue: "Ana", ValueKind: operations.ValueText}}}},
		},
		report: operations.Report{
			ImportID: id, State: operations.ImportCompleted, Inserted: 1, Updated: 2, Linked: 3,
			Skipped: 4, Errored: 5, Conflicted: 6, Decisions: 7,
			Rows: []operations.ReportRow{{SheetIndex: 0, RowNumber: 2, Outcome: operations.OutcomeLinked}},
		},
		exported: operations.Export{ID: id, ActorUserID: actorID, Module: operations.ModuleProfiles, State: operations.ExportCompleted, ObjectKey: "operations/private/export.xlsx", RiverJobID: 456, Filename: "profiles.xlsx", ExpiresAt: now.Add(time.Hour), Version: 2, CreatedAt: now, UpdatedAt: now},
		bulk:     operations.BulkDeleteResult{Module: operations.ModuleProfiles, Deleted: 1},
	}
	handler := operationsHTTPFixture(t, service)

	catalog := operationRequest(t, handler, http.MethodGet, "/api/v1/operations/catalog", "")
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), `"can_duplicate":true`) || !strings.Contains(catalog.Body.String(), `"can_delete":true`) {
		t.Fatalf("catalog response = %d, %s", catalog.Code, catalog.Body.String())
	}

	response := operationRequest(t, handler, http.MethodGet, "/api/v1/operations/imports/"+id.String(), "")
	if response.Code != http.StatusOK {
		t.Fatalf("import response = %d, %s", response.Code, response.Body.String())
	}
	for _, forbidden := range []string{"private/secret", "object_key", "river_job", "content_sha256"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("import response exposed %q: %s", forbidden, response.Body.String())
		}
	}
	if !strings.Contains(response.Body.String(), `"raw_value":"Ana"`) {
		t.Fatalf("bounded preview missing from response: %s", response.Body.String())
	}

	report := operationRequest(t, handler, http.MethodGet, "/api/v1/operations/imports/"+id.String()+"/report", "")
	if report.Code != http.StatusOK || !strings.Contains(report.Body.String(), `"linked":3`) || !strings.Contains(report.Body.String(), `"outcome":"LINKED"`) {
		t.Fatalf("report response = %d, %s", report.Code, report.Body.String())
	}
	for _, forbidden := range []string{"private/secret", "object_key", "raw_value", "river_job", "signed"} {
		if strings.Contains(report.Body.String(), forbidden) {
			t.Fatalf("report response exposed %q: %s", forbidden, report.Body.String())
		}
	}

	bulkBody := `{"module":"PROFILES","items":[{"id":"` + id.String() + `","version":2}],"confirmation":"Confirmar"}`
	bulk := operationRequest(t, handler, http.MethodPost, "/api/v1/operations/bulk-delete", bulkBody)
	if bulk.Code != http.StatusOK || strings.TrimSpace(bulk.Body.String()) != `{"module":"PROFILES","deleted":1}` {
		t.Fatalf("bulk response = %d, %s", bulk.Code, bulk.Body.String())
	}
	if service.lastModule != operations.ModuleProfiles || len(service.lastItems) != 1 || service.lastItems[0].Version != 2 {
		t.Fatalf("bulk service input = %q, %#v", service.lastModule, service.lastItems)
	}
}

func TestOperationsHTTPRejectsInvalidIdentifiersAndMapsStableErrors(t *testing.T) {
	service := &fakeOperationsService{}
	handler := operationsHTTPFixture(t, service)
	invalid := operationRequest(t, handler, http.MethodGet, "/api/v1/operations/imports/not-a-uuid", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid identifier status = %d", invalid.Code)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   ErrorCode
	}{
		{name: "forbidden", err: operations.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: ErrorCodeForbidden},
		{name: "conflict", err: operations.ErrStalePreview, wantStatus: http.StatusConflict, wantCode: ErrorCodeConflict},
		{name: "rate", err: operations.ErrRateLimited, wantStatus: http.StatusTooManyRequests, wantCode: ErrorCodeRateLimited},
		{name: "mapping", err: operations.ErrInvalidMapping, wantStatus: http.StatusUnprocessableEntity, wantCode: ErrorCodeValidation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service.err = test.err
			response := operationRequest(t, handler, http.MethodGet, "/api/v1/operations/catalog", "")
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			var payload errorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Error.Code != test.wantCode {
				t.Fatalf("payload = %#v, error = %v", payload, err)
			}
		})
	}
}

func TestOperationsHTTPRequiresAuthentication(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}, Operations: &fakeOperationsService{}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/operations/catalog", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func operationRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
