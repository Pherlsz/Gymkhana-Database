package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

type operationsService interface {
	Catalog(context.Context, auth.Session) ([]operations.ModuleCatalog, error)
	CreateImport(context.Context, auth.Session, operations.CreateImportInput, string) (operations.UploadGrant, error)
	ConfirmImport(context.Context, auth.Session, operations.Identifier, string) (operations.Import, error)
	GetImport(context.Context, auth.Session, operations.Identifier) (operations.Import, error)
	GetReport(context.Context, auth.Session, operations.Identifier) (operations.Report, error)
	ListImports(context.Context, auth.Session, operations.ListOptions) (operations.ImportPage, error)
	SelectSheet(context.Context, auth.Session, operations.Identifier, int64, int, string) (operations.Import, error)
	SaveMapping(context.Context, auth.Session, operations.Identifier, int64, []operations.MappingInput, string) (operations.Import, error)
	Preview(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error)
	SaveDecisions(context.Context, auth.Session, operations.Identifier, int64, []operations.DecisionInput, string) (operations.Import, error)
	Execute(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error)
	CancelImport(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error)
	CreateExport(context.Context, auth.Session, operations.Module, string, string) (operations.Export, error)
	GetExport(context.Context, auth.Session, operations.Identifier) (operations.Export, error)
	ListExports(context.Context, auth.Session, operations.ListOptions) (operations.ExportPage, error)
	DownloadExport(context.Context, auth.Session, operations.Identifier, string) (operations.DownloadGrant, error)
	BulkDelete(context.Context, auth.Session, operations.Module, []operations.BulkItem, string, string) (operations.BulkDeleteResult, error)
}

type operationCreateImportRequest struct {
	Module           operations.Module `json:"module"`
	OriginalFilename string            `json:"original_filename"`
	DeclaredSize     int64             `json:"declared_size"`
	IdempotencyKey   string            `json:"idempotency_key"`
}

type operationVersionRequest struct {
	Version int64 `json:"version"`
}

type operationSheetRequest struct {
	Version    int64 `json:"version"`
	SheetIndex int   `json:"sheet_index"`
}

type operationMappingRequest struct {
	Version int64                     `json:"version"`
	Mapping []operations.MappingInput `json:"mapping"`
}

type operationDecisionRequest struct {
	Version   int64                      `json:"version"`
	Decisions []operationDecisionPayload `json:"decisions"`
}

type operationDecisionPayload struct {
	RowNumber int               `json:"row_number"`
	Action    operations.Action `json:"action"`
	TargetID  string            `json:"target_id,omitempty"`
	Version   int64             `json:"target_version,omitempty"`
}

type operationCreateExportRequest struct {
	Module         operations.Module `json:"module"`
	IdempotencyKey string            `json:"idempotency_key"`
}

type operationBulkDeleteRequest struct {
	Module       operations.Module      `json:"module"`
	Items        []operationBulkPayload `json:"items"`
	Confirmation string                 `json:"confirmation"`
}

type operationBulkPayload struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type operationCatalogResponse struct {
	Modules []operationCatalogModule `json:"modules"`
	Limits  operationLimitsResponse  `json:"limits"`
}

type operationCatalogModule struct {
	ID            operations.Module `json:"id"`
	Label         string            `json:"label"`
	CanImport     bool              `json:"can_import"`
	CanExport     bool              `json:"can_export"`
	CanDuplicate  bool              `json:"can_duplicate"`
	CanDelete     bool              `json:"can_delete"`
	CanBulkDelete bool              `json:"can_bulk_delete"`
	Fields        []operationField  `json:"fields"`
}

type operationField struct {
	ID         string               `json:"id"`
	Label      string               `json:"label"`
	Kind       operations.FieldKind `json:"kind"`
	Required   bool                 `json:"required"`
	Importable bool                 `json:"importable"`
	Exportable bool                 `json:"exportable"`
}

type operationLimitsResponse struct {
	MaximumFileSize      int64 `json:"maximum_file_size"`
	MaximumRows          int   `json:"maximum_rows"`
	MaximumColumns       int   `json:"maximum_columns"`
	MaximumCells         int   `json:"maximum_cells"`
	MaximumPreviewRows   int   `json:"maximum_preview_rows"`
	MaximumBulkSelection int   `json:"maximum_bulk_selection"`
}

type operationImportResponse struct {
	ID                   string                    `json:"id"`
	Module               operations.Module         `json:"module"`
	SourceKind           operations.SourceKind     `json:"source_kind"`
	OriginalFilename     *string                   `json:"original_filename,omitempty"`
	DeclaredSize         *int64                    `json:"declared_size,omitempty"`
	ActualSize           *int64                    `json:"actual_size,omitempty"`
	State                operations.ImportState    `json:"state"`
	Stage                operations.Stage          `json:"stage"`
	SelectedSheetIndex   *int                      `json:"selected_sheet_index,omitempty"`
	MappingVersion       int64                     `json:"mapping_version"`
	UnresolvedCount      int                       `json:"unresolved_count"`
	ValidationErrorCount int                       `json:"validation_error_count"`
	InsertedCount        int                       `json:"inserted_count"`
	UpdatedCount         int                       `json:"updated_count"`
	LinkedCount          int                       `json:"linked_count"`
	SkippedCount         int                       `json:"skipped_count"`
	ErroredCount         int                       `json:"errored_count"`
	ConflictedCount      int                       `json:"conflicted_count"`
	ErrorCode            string                    `json:"error_code,omitempty"`
	ExpiresAt            time.Time                 `json:"expires_at"`
	CancelledAt          *time.Time                `json:"cancelled_at,omitempty"`
	CompletedAt          *time.Time                `json:"completed_at,omitempty"`
	Version              int64                     `json:"version"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
	Sheets               []operationSheetResponse  `json:"sheets"`
	Columns              []operationColumnResponse `json:"columns"`
	Preview              []operationRowResponse    `json:"preview"`
}

type operationSheetResponse struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	RowCount    int    `json:"row_count"`
	ColumnCount int    `json:"column_count"`
}

type operationColumnResponse struct {
	SheetIndex   int    `json:"sheet_index"`
	SourceColumn int    `json:"source_column"`
	SourceHeader string `json:"source_header"`
	TargetField  string `json:"target_field,omitempty"`
}

type operationRowResponse struct {
	SheetIndex           int                       `json:"sheet_index"`
	RowNumber            int                       `json:"row_number"`
	ProposedAction       operations.Action         `json:"proposed_action"`
	Decision             operations.Action         `json:"decision,omitempty"`
	TargetID             string                    `json:"target_id,omitempty"`
	TargetVersion        int64                     `json:"target_version,omitempty"`
	ValidationErrorCount int                       `json:"validation_error_count"`
	DecisionRequired     bool                      `json:"decision_required"`
	Cells                []operationCellResponse   `json:"cells"`
	Outcome              *operationOutcomeResponse `json:"outcome,omitempty"`
}

type operationCellResponse struct {
	SourceColumn   int                  `json:"source_column"`
	RawValue       string               `json:"raw_value"`
	ValueKind      operations.ValueKind `json:"value_kind"`
	FormulaPresent bool                 `json:"formula_present"`
	ValidationCode string               `json:"validation_code,omitempty"`
}

type operationOutcomeResponse struct {
	Kind        operations.OutcomeKind `json:"kind"`
	TargetID    string                 `json:"target_id,omitempty"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	CommittedAt time.Time              `json:"committed_at"`
}

type operationUploadGrantResponse struct {
	Import    operationImportResponse `json:"import"`
	UploadURL string                  `json:"upload_url"`
	Method    string                  `json:"method"`
	Headers   map[string]string       `json:"headers"`
	ExpiresAt time.Time               `json:"expires_at"`
}

type operationImportPageResponse struct {
	Imports []operationImportResponse `json:"imports"`
	Total   int64                     `json:"total"`
	Limit   int                       `json:"limit"`
	Offset  int                       `json:"offset"`
}

type operationReportResponse struct {
	ImportID         string                 `json:"import_id"`
	State            operations.ImportState `json:"state"`
	Inserted         int                    `json:"inserted"`
	Updated          int                    `json:"updated"`
	Linked           int                    `json:"linked"`
	Skipped          int                    `json:"skipped"`
	Errored          int                    `json:"errored"`
	Conflicted       int                    `json:"conflicted"`
	Decisions        int                    `json:"decisions"`
	Unresolved       int                    `json:"unresolved"`
	ValidationErrors int                    `json:"validation_errors"`
	Rows             []operationReportRow   `json:"rows"`
}

type operationReportRow struct {
	SheetIndex int                    `json:"sheet_index"`
	RowNumber  int                    `json:"row_number"`
	Outcome    operations.OutcomeKind `json:"outcome,omitempty"`
	ErrorCode  string                 `json:"error_code,omitempty"`
}

type operationExportResponse struct {
	ID          string                 `json:"id"`
	Module      operations.Module      `json:"module"`
	State       operations.ExportState `json:"state"`
	Filename    string                 `json:"filename"`
	RowCount    int                    `json:"row_count"`
	ByteSize    int64                  `json:"byte_size"`
	ErrorCode   string                 `json:"error_code,omitempty"`
	ExpiresAt   time.Time              `json:"expires_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Version     int64                  `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type operationExportPageResponse struct {
	Exports []operationExportResponse `json:"exports"`
	Total   int64                     `json:"total"`
	Limit   int                       `json:"limit"`
	Offset  int                       `json:"offset"`
}

type operationDownloadResponse struct {
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expires_at"`
}

type operationBulkDeleteResponse struct {
	Module  operations.Module `json:"module"`
	Deleted int               `json:"deleted"`
}

func registerOperationsRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service operationsService) {
	mux.HandleFunc("GET /api/v1/operations/catalog", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		catalog, err := service.Catalog(r.Context(), actor)
		if err != nil {
			writeOperationsError(w, r, logger, "read operations catalog", err)
			return
		}
		writeJSON(w, http.StatusOK, operationCatalogFromDomain(catalog))
	})

	mux.HandleFunc("POST /api/v1/operations/imports", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationCreateImportRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		grant, err := service.CreateImport(r.Context(), actor, operations.CreateImportInput{
			Module: request.Module, OriginalFilename: request.OriginalFilename,
			DeclaredSize: request.DeclaredSize, IdempotencyKey: request.IdempotencyKey,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "create operation import", err)
			return
		}
		writeJSON(w, http.StatusCreated, operationUploadGrantResponse{
			Import: operationImportFromDomain(grant.Import), UploadURL: grant.UploadURL,
			Method: grant.Method, Headers: grant.Headers, ExpiresAt: grant.ExpiresAt,
		})
	})

	mux.HandleFunc("POST /api/v1/operations/imports/{import_id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.ConfirmImport(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "confirm operation import", err)
			return
		}
		writeJSON(w, http.StatusAccepted, operationImportFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/operations/imports", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		options, problem := operationListOptions(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListImports(r.Context(), actor, options)
		if err != nil {
			writeOperationsError(w, r, logger, "list operation imports", err)
			return
		}
		response := operationImportPageResponse{Total: page.Total, Limit: page.Limit, Offset: page.Offset, Imports: make([]operationImportResponse, 0, len(page.Imports))}
		for _, value := range page.Imports {
			response.Imports = append(response.Imports, operationImportFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/operations/imports/{import_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.GetImport(r.Context(), actor, id)
		if err != nil {
			writeOperationsError(w, r, logger, "get operation import", err)
			return
		}
		writeJSON(w, http.StatusOK, operationImportFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/operations/imports/{import_id}/report", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.GetReport(r.Context(), actor, id)
		if err != nil {
			writeOperationsError(w, r, logger, "get operation import report", err)
			return
		}
		rows := make([]operationReportRow, 0, len(value.Rows))
		for _, row := range value.Rows {
			rows = append(rows, operationReportRow{SheetIndex: row.SheetIndex, RowNumber: row.RowNumber, Outcome: row.Outcome, ErrorCode: row.ErrorCode})
		}
		writeJSON(w, http.StatusOK, operationReportResponse{
			ImportID: value.ImportID.String(), State: value.State, Inserted: value.Inserted,
			Updated: value.Updated, Linked: value.Linked, Skipped: value.Skipped, Errored: value.Errored,
			Conflicted: value.Conflicted, Decisions: value.Decisions, Unresolved: value.Unresolved,
			ValidationErrors: value.ValidationErrors, Rows: rows,
		})
	})

	mux.HandleFunc("PUT /api/v1/operations/imports/{import_id}/sheet", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationSheetRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.SelectSheet(r.Context(), actor, id, request.Version, request.SheetIndex, requestIDFromContext(r.Context()))
		writeOperationImportResult(w, r, logger, "select operation import sheet", value, err)
	})

	mux.HandleFunc("PUT /api/v1/operations/imports/{import_id}/mapping", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationMappingRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.SaveMapping(r.Context(), actor, id, request.Version, request.Mapping, requestIDFromContext(r.Context()))
		writeOperationImportResult(w, r, logger, "save operation import mapping", value, err)
	})

	mux.HandleFunc("POST /api/v1/operations/imports/{import_id}/preview", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationVersionRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.Preview(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		writeOperationImportResult(w, r, logger, "preview operation import", value, err)
	})

	mux.HandleFunc("PUT /api/v1/operations/imports/{import_id}/decisions", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationDecisionRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		decisions, problem := operationDecisions(request.Decisions)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.SaveDecisions(r.Context(), actor, id, request.Version, decisions, requestIDFromContext(r.Context()))
		writeOperationImportResult(w, r, logger, "save operation import decisions", value, err)
	})

	mux.HandleFunc("POST /api/v1/operations/imports/{import_id}/execute", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationVersionRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.Execute(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "execute operation import", err)
			return
		}
		writeJSON(w, http.StatusAccepted, operationImportFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/operations/imports/{import_id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationImportActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationVersionRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.CancelImport(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		writeOperationImportResult(w, r, logger, "cancel operation import", value, err)
	})

	registerOperationExportRoutes(mux, logger, authentication, service)
}

func registerOperationExportRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service operationsService) {
	mux.HandleFunc("POST /api/v1/operations/exports", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationCreateExportRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		value, err := service.CreateExport(r.Context(), actor, request.Module, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "create operation export", err)
			return
		}
		writeJSON(w, http.StatusAccepted, operationExportFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/operations/exports", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		options, problem := operationListOptions(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListExports(r.Context(), actor, options)
		if err != nil {
			writeOperationsError(w, r, logger, "list operation exports", err)
			return
		}
		response := operationExportPageResponse{Total: page.Total, Limit: page.Limit, Offset: page.Offset, Exports: make([]operationExportResponse, 0, len(page.Exports))}
		for _, value := range page.Exports {
			response.Exports = append(response.Exports, operationExportFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/operations/exports/{export_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationExportActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.GetExport(r.Context(), actor, id)
		if err != nil {
			writeOperationsError(w, r, logger, "get operation export", err)
			return
		}
		writeJSON(w, http.StatusOK, operationExportFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/operations/exports/{export_id}/download", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := operationExportActor(w, r, authentication, service)
		if !ok {
			return
		}
		grant, err := service.DownloadExport(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "download operation export", err)
			return
		}
		writeJSON(w, http.StatusOK, operationDownloadResponse{URL: grant.URL, Method: grant.Method, ExpiresAt: grant.ExpiresAt})
	})

	mux.HandleFunc("POST /api/v1/operations/bulk-delete", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := operationsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request operationBulkDeleteRequest
		if !decodeOperationRequest(w, r, &request) {
			return
		}
		items, problem := operationBulkItems(request.Items)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.BulkDelete(r.Context(), actor, request.Module, items, request.Confirmation, requestIDFromContext(r.Context()))
		if err != nil {
			writeOperationsError(w, r, logger, "bulk delete operation records", err)
			return
		}
		writeJSON(w, http.StatusOK, operationBulkDeleteResponse{Module: value.Module, Deleted: value.Deleted})
	})
}

func operationsActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service operationsService) (auth.Session, bool) {
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "Operações estão indisponíveis"})
		return auth.Session{}, false
	}
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	return actor, true
}

func operationImportActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service operationsService) (auth.Session, operations.Identifier, bool) {
	actor, ok := operationsActor(w, r, authentication, service)
	if !ok {
		return auth.Session{}, operations.Identifier{}, false
	}
	id, err := operations.ParseIdentifier(r.PathValue("import_id"))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de importação inválido"})
		return auth.Session{}, operations.Identifier{}, false
	}
	return actor, id, true
}

func operationExportActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service operationsService) (auth.Session, operations.Identifier, bool) {
	actor, ok := operationsActor(w, r, authentication, service)
	if !ok {
		return auth.Session{}, operations.Identifier{}, false
	}
	id, err := operations.ParseIdentifier(r.PathValue("export_id"))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de exportação inválido"})
		return auth.Session{}, operations.Identifier{}, false
	}
	return actor, id, true
}

func decodeOperationRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	if problem := DecodeJSON(w, r, destination); problem != nil {
		writeProblem(w, r, *problem)
		return false
	}
	return true
}

func operationListOptions(r *http.Request) (operations.ListOptions, *Problem) {
	options := operations.ListOptions{Limit: 50}
	for key, destination := range map[string]*int{"limit": &options.Limit, "offset": &options.Offset} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return operations.ListOptions{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação de operações inválida"}
		}
		*destination = value
	}
	if options.Limit < 1 || options.Limit > 100 || options.Offset < 0 || options.Offset > 10_000 {
		return operations.ListOptions{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação de operações fora dos limites"}
	}
	return options, nil
}

func operationDecisions(payload []operationDecisionPayload) ([]operations.DecisionInput, *Problem) {
	result := make([]operations.DecisionInput, 0, len(payload))
	for _, value := range payload {
		decision := operations.DecisionInput{RowNumber: value.RowNumber, Action: value.Action, Version: value.Version}
		if strings.TrimSpace(value.TargetID) != "" {
			id, err := operations.ParseIdentifier(value.TargetID)
			if err != nil {
				return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Decisão contém identificador inválido"}
			}
			decision.TargetID = &id
		}
		result = append(result, decision)
	}
	return result, nil
}

func operationBulkItems(payload []operationBulkPayload) ([]operations.BulkItem, *Problem) {
	result := make([]operations.BulkItem, 0, len(payload))
	for _, value := range payload {
		id, err := operations.ParseIdentifier(value.ID)
		if err != nil {
			return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Seleção contém identificador inválido"}
		}
		result = append(result, operations.BulkItem{ID: id, Version: value.Version})
	}
	return result, nil
}

func operationCatalogFromDomain(catalog []operations.ModuleCatalog) operationCatalogResponse {
	response := operationCatalogResponse{Modules: make([]operationCatalogModule, 0, len(catalog)), Limits: operationLimitsResponse{
		MaximumFileSize: operations.MaximumFileSize, MaximumRows: operations.MaximumRows,
		MaximumColumns: operations.MaximumColumns, MaximumCells: operations.MaximumCells,
		MaximumPreviewRows: operations.MaximumPreviewRows, MaximumBulkSelection: operations.MaximumBulkSelection,
	}}
	for _, module := range catalog {
		converted := operationCatalogModule{ID: module.ID, Label: module.Label, CanImport: module.CanImport, CanExport: module.CanExport, CanDuplicate: module.CanDuplicate, CanDelete: module.CanDelete, CanBulkDelete: module.CanBulkDelete, Fields: make([]operationField, 0, len(module.Fields))}
		for _, field := range module.Fields {
			converted.Fields = append(converted.Fields, operationField{ID: field.ID, Label: field.Label, Kind: field.Kind, Required: field.Required, Importable: field.Importable, Exportable: field.Exportable})
		}
		response.Modules = append(response.Modules, converted)
	}
	return response
}

func operationImportFromDomain(value operations.Import) operationImportResponse {
	response := operationImportResponse{
		ID: value.ID.String(), Module: value.Module, SourceKind: value.SourceKind,
		State: value.State, Stage: value.Stage, SelectedSheetIndex: value.SelectedSheetIndex,
		MappingVersion: value.MappingVersion, UnresolvedCount: value.UnresolvedCount,
		ValidationErrorCount: value.ValidationErrorCount, InsertedCount: value.InsertedCount,
		UpdatedCount: value.UpdatedCount, LinkedCount: value.LinkedCount, SkippedCount: value.SkippedCount, ErroredCount: value.ErroredCount,
		ConflictedCount: value.ConflictedCount, ErrorCode: value.ErrorCode, ExpiresAt: value.ExpiresAt,
		CancelledAt: value.CancelledAt, CompletedAt: value.CompletedAt, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Sheets: make([]operationSheetResponse, 0, len(value.Sheets)),
		Columns: make([]operationColumnResponse, 0, len(value.Columns)), Preview: make([]operationRowResponse, 0, len(value.Preview)),
	}
	if value.SourceKind == operations.SourceXLSX {
		response.OriginalFilename = &value.OriginalFilename
		response.DeclaredSize = &value.DeclaredSize
		if value.ActualSize > 0 {
			response.ActualSize = &value.ActualSize
		}
	}
	for _, sheet := range value.Sheets {
		response.Sheets = append(response.Sheets, operationSheetResponse{Index: sheet.Index, Name: sheet.Name, RowCount: sheet.RowCount, ColumnCount: sheet.ColumnCount})
	}
	for _, column := range value.Columns {
		response.Columns = append(response.Columns, operationColumnResponse{SheetIndex: column.SheetIndex, SourceColumn: column.SourceColumn, SourceHeader: column.SourceHeader, TargetField: column.TargetField})
	}
	for _, row := range value.Preview {
		converted := operationRowResponse{SheetIndex: row.SheetIndex, RowNumber: row.RowNumber, ProposedAction: row.ProposedAction, Decision: row.Decision, TargetVersion: row.TargetVersion, ValidationErrorCount: row.ValidationErrorCount, DecisionRequired: row.DecisionRequired, Cells: make([]operationCellResponse, 0, len(row.Cells))}
		if row.TargetID != nil {
			converted.TargetID = row.TargetID.String()
		}
		for _, cell := range row.Cells {
			converted.Cells = append(converted.Cells, operationCellResponse{SourceColumn: cell.SourceColumn, RawValue: cell.RawValue, ValueKind: cell.ValueKind, FormulaPresent: cell.FormulaPresent, ValidationCode: cell.ValidationCode})
		}
		if row.Outcome != nil {
			converted.Outcome = &operationOutcomeResponse{Kind: row.Outcome.Kind, ErrorCode: row.Outcome.ErrorCode, CommittedAt: row.Outcome.CommittedAt}
			if row.Outcome.TargetID != nil {
				converted.Outcome.TargetID = row.Outcome.TargetID.String()
			}
		}
		response.Preview = append(response.Preview, converted)
	}
	return response
}

func operationExportFromDomain(value operations.Export) operationExportResponse {
	return operationExportResponse{
		ID: value.ID.String(), Module: value.Module, State: value.State, Filename: value.Filename,
		RowCount: value.RowCount, ByteSize: value.ByteSize, ErrorCode: value.ErrorCode,
		ExpiresAt: value.ExpiresAt, CompletedAt: value.CompletedAt, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func writeOperationImportResult(w http.ResponseWriter, r *http.Request, logger *slog.Logger, action string, value operations.Import, err error) {
	if err != nil {
		writeOperationsError(w, r, logger, action, err)
		return
	}
	writeJSON(w, http.StatusOK, operationImportFromDomain(value))
}

func writeOperationsError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, action string, err error) {
	switch {
	case errors.Is(err, operations.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para esta operação"})
	case errors.Is(err, operations.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Operação não encontrada"})
	case errors.Is(err, operations.ErrConflict), errors.Is(err, operations.ErrStalePreview):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A operação mudou. Atualize os dados e tente novamente"})
	case errors.Is(err, operations.ErrExpired), errors.Is(err, operations.ErrCancelled), errors.Is(err, operations.ErrDecisionRequired), errors.Is(err, operations.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A operação não está disponível neste estado"})
	case errors.Is(err, operations.ErrRateLimited), errors.Is(err, operations.ErrQuotaExceeded):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de operações atingido. Aguarde ou conclua uma operação ativa"})
	case errors.Is(err, operations.ErrInvalidMapping):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "O mapeamento de colunas é inválido"})
	case errors.Is(err, operations.ErrUnsupportedWorkbook), errors.Is(err, operations.ErrWorkbookLimit):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "A planilha não atende aos limites ou ao formato suportado"})
	case errors.Is(err, operations.ErrInvalidConfirmation):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "A confirmação da exclusão é inválida"})
	case errors.Is(err, operations.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Os dados da operação são inválidos"})
	default:
		logger.Error(action, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação"})
	}
}
