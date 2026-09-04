package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
	"github.com/jackc/pgx/v5/pgxpool"
)

type billService interface {
	ListTypes(context.Context, auth.Session, bill.TypeListOptions) (bill.TypePage, error)
	GetType(context.Context, auth.Session, bill.Identifier) (bill.TypeDefinition, error)
	CreateType(context.Context, auth.Session, bill.TypeValues, string) (bill.TypeDefinition, error)
	UpdateType(context.Context, auth.Session, bill.Identifier, int64, bill.TypeValues, string) (bill.TypeDefinition, error)
	DeleteType(context.Context, auth.Session, bill.Identifier, int64, string, string) error
	List(context.Context, auth.Session, bill.ListOptions) (bill.Page, error)
	Get(context.Context, auth.Session, bill.Identifier) (bill.Bill, error)
	Create(context.Context, auth.Session, bill.Values, string) (bill.Bill, error)
	Update(context.Context, auth.Session, bill.Identifier, int64, bill.Values, string) (bill.Bill, error)
	Duplicate(context.Context, auth.Session, bill.Identifier, string) (bill.Bill, error)
	Delete(context.Context, auth.Session, bill.Identifier, int64, string, string) error
	AssignCurrentUse(context.Context, auth.Session, bill.Identifier, profile.Identifier, string) (bill.CurrentUse, error)
	ReturnCurrentUse(context.Context, auth.Session, bill.Identifier, string) error
}

type billTypeValuesRequest struct {
	TechnicalKey string `json:"technical_key"`
	Label        string `json:"label"`
	Active       bool   `json:"active"`
}

type updateBillTypeRequest struct {
	TechnicalKey string `json:"technical_key"`
	Label        string `json:"label"`
	Active       bool   `json:"active"`
	Version      int64  `json:"version"`
}

type billValuesRequest struct {
	OwnerProfileID    string           `json:"owner_profile_id"`
	OwnerName         string           `json:"owner_name"`
	BillTypeID        string           `json:"bill_type_id"`
	PrintedHolderName string           `json:"printed_holder_name"`
	PrintedAddress    string           `json:"printed_address"`
	Reference         string           `json:"reference_value"`
	Competence        string           `json:"competence"`
	Amount            string           `json:"amount"`
	Currency          string           `json:"currency"`
	Notes             string           `json:"notes"`
	Medium            bill.Medium      `json:"medium"`
	IdleCustody       bill.IdleCustody `json:"idle_custody"`
}

type updateBillRequest struct {
	OwnerProfileID    string           `json:"owner_profile_id"`
	BillTypeID        string           `json:"bill_type_id"`
	PrintedHolderName string           `json:"printed_holder_name"`
	PrintedAddress    string           `json:"printed_address"`
	Reference         string           `json:"reference_value"`
	Competence        string           `json:"competence"`
	Amount            string           `json:"amount"`
	Currency          string           `json:"currency"`
	Notes             string           `json:"notes"`
	Medium            bill.Medium      `json:"medium"`
	IdleCustody       bill.IdleCustody `json:"idle_custody"`
	Version           int64            `json:"version"`
}

type deleteBillResourceRequest struct {
	Version      int64  `json:"version"`
	Confirmation string `json:"confirmation"`
}
type assignBillCurrentUseRequest struct {
	HolderProfileID string `json:"holder_profile_id"`
}

type billTypeResponse struct {
	ID           string    `json:"id"`
	TechnicalKey string    `json:"technical_key"`
	Label        string    `json:"label"`
	Active       bool      `json:"active"`
	Count        int64     `json:"count"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type billCurrentUseResponse struct {
	HolderProfileID string    `json:"holder_profile_id"`
	HolderFullName  string    `json:"holder_full_name,omitempty"`
	AssignedAt      time.Time `json:"assigned_at"`
}
type billResponse struct {
	ID                string                  `json:"id"`
	OwnerProfileID    string                  `json:"owner_profile_id"`
	OwnerFullName     string                  `json:"owner_full_name"`
	BillTypeID        string                  `json:"bill_type_id"`
	PrintedHolderName string                  `json:"printed_holder_name"`
	PrintedAddress    string                  `json:"printed_address"`
	Reference         string                  `json:"reference_value"`
	Competence        string                  `json:"competence"`
	Amount            string                  `json:"amount"`
	Currency          string                  `json:"currency"`
	Notes             string                  `json:"notes"`
	Medium            bill.Medium             `json:"medium"`
	IdleCustody       bill.IdleCustody        `json:"idle_custody,omitempty"`
	Status            bill.Status             `json:"status,omitempty"`
	Type              billTypeResponse        `json:"type"`
	CurrentUse        *billCurrentUseResponse `json:"current_use"`
	CustomValues      map[string]string       `json:"custom_values"`
	Version           int64                   `json:"version"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}
type billPageMeta struct {
	Total     int64  `json:"total"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
	SortField string `json:"sort_field"`
	SortOrder string `json:"sort_order"`
}
type billTypePageResponse struct {
	Types []billTypeResponse `json:"types"`
	Page  billPageMeta       `json:"page"`
}
type billPageResponse struct {
	Bills []billResponse `json:"bills"`
	Page  billPageMeta   `json:"page"`
}

func registerBillRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service billService, search searchService, pool *pgxpool.Pool) {
	mux.HandleFunc("GET /api/v1/bill-types", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		limit, problem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		offset, problem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		active, problem := parseOptionalBillBool(r.URL.Query().Get("active"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListTypes(r.Context(), actor, bill.TypeListOptions{Limit: limit, Offset: offset, SortField: bill.TypeSortField(r.URL.Query().Get("sort")), SortOrder: bill.SortOrder(r.URL.Query().Get("order")), Filters: bill.TypeFilters{Label: r.URL.Query().Get("label"), Active: active}})
		if err != nil {
			writeBillError(w, r, logger, "list bill types", err)
			return
		}
		response := billTypePageResponse{Types: make([]billTypeResponse, 0, len(page.Types)), Page: billPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Types {
			response.Types = append(response.Types, billTypeFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	}))

	mux.HandleFunc("POST /api/v1/bill-types", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request billTypeValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		created, err := service.CreateType(r.Context(), actor, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "create bill type", err)
			return
		}
		writeJSON(w, http.StatusCreated, billTypeFromDomain(created))
	}))

	mux.HandleFunc("GET /api/v1/bill-types/{bill_type_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_type_id"), "O identificador do tipo de conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetType(r.Context(), actor, id)
		if err != nil {
			writeBillError(w, r, logger, "get bill type", err)
			return
		}
		writeJSON(w, http.StatusOK, billTypeFromDomain(value))
	}))

	mux.HandleFunc("PUT /api/v1/bill-types/{bill_type_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_type_id"), "O identificador do tipo de conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request updateBillTypeRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := service.UpdateType(r.Context(), actor, id, request.Version, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "update bill type", err)
			return
		}
		writeJSON(w, http.StatusOK, billTypeFromDomain(updated))
	}))

	mux.HandleFunc("DELETE /api/v1/bill-types/{bill_type_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_type_id"), "O identificador do tipo de conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request deleteBillResourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteType(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeBillError(w, r, logger, "delete bill type", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("GET /api/v1/bills", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		options, problem := billListOptionsFromRequest(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		restrict, ids, searchProblem := applySearchQ(r, actor, search, searchdomain.ModuleBills)
		if searchProblem != nil {
			writeProblem(w, r, *searchProblem)
			return
		}
		options.Filters.RestrictIDs = restrict
		options.Filters.IDFilter = billIDsFromSearch(ids)
		page, err := service.List(r.Context(), actor, options)
		if err != nil {
			writeBillError(w, r, logger, "list bills", err)
			return
		}
		response := billPageResponse{Bills: make([]billResponse, 0, len(page.Bills)), Page: billPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Bills {
			response.Bills = append(response.Bills, billFromDomain(value))
		}
		enrichBillList(r.Context(), pool, logger, response.Bills)
		writeJSON(w, http.StatusOK, response)
	}))

	mux.HandleFunc("POST /api/v1/bills", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request billValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domainValues()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		created, err := service.Create(r.Context(), actor, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "create bill", err)
			return
		}
		writeJSON(w, http.StatusCreated, billFromDomain(created))
	}))

	mux.HandleFunc("GET /api/v1/bills/{bill_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Get(r.Context(), actor, id)
		if err != nil {
			writeBillError(w, r, logger, "get bill", err)
			return
		}
		writeJSON(w, http.StatusOK, billFromDomain(value))
	}))

	mux.HandleFunc("PUT /api/v1/bills/{bill_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request updateBillRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domainValues()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := service.Update(r.Context(), actor, id, request.Version, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "update bill", err)
			return
		}
		writeJSON(w, http.StatusOK, billFromDomain(updated))
	}))

	mux.HandleFunc("POST /api/v1/bills/{bill_id}/duplicate", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		duplicated, err := service.Duplicate(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "duplicate bill", err)
			return
		}
		writeJSON(w, http.StatusCreated, billFromDomain(duplicated))
	}))

	mux.HandleFunc("DELETE /api/v1/bills/{bill_id}", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request deleteBillResourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.Delete(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeBillError(w, r, logger, "delete bill", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("PUT /api/v1/bills/{bill_id}/current-use", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request assignBillCurrentUseRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		holder, err := profile.ParseIdentifier(request.HolderProfileID)
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa em uso é inválido"})
			return
		}
		currentUse, err := service.AssignCurrentUse(r.Context(), actor, id, holder, requestIDFromContext(r.Context()))
		if err != nil {
			writeBillError(w, r, logger, "assign bill current use", err)
			return
		}
		writeJSON(w, http.StatusOK, billCurrentUseResponse{HolderProfileID: currentUse.HolderProfileID.String(), AssignedAt: currentUse.AssignedAt})
	}))

	mux.HandleFunc("DELETE /api/v1/bills/{bill_id}/current-use", requireCapability(auth.CapDataTables, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := billActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseBillIdentifier(r.PathValue("bill_id"), "O identificador do conta/comprovante é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.ReturnCurrentUse(r.Context(), actor, id, requestIDFromContext(r.Context())); err != nil {
			writeBillError(w, r, logger, "return bill current use", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}
