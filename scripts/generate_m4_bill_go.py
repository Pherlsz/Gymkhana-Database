#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content.rstrip() + "\n", encoding="utf-8")


def replace_once(text: str, old: str, new: str) -> str:
    if old not in text:
        raise RuntimeError(f"expected fragment not found: {old[:100]!r}")
    return text.replace(old, new, 1)


def to_bill(text: str) -> str:
    for old, new in (
        ("Documents", "Bills"), ("Document", "Bill"),
        ("documents", "bills"), ("document", "bill"),
        ("DOCUMENT", "BILL"),
        ("Documentos", "Contas"), ("Documento", "Conta"),
        ("documentos", "contas"), ("documento", "conta"),
    ):
        text = text.replace(old, new)
    return text


def generate_service() -> None:
    text = to_bill(read("internal/document/service.go"))
    text = text.replace('"fmt"\n', '"fmt"\n\t"strings"\n')
    text = text.replace("options.SortField = SortIdentifier", "options.SortField = SortReference")
    text = text.replace(
        "options.Filters.Identifier = normalize.SearchText(options.Filters.Identifier)",
        'options.Filters.Reference = normalize.SearchText(options.Filters.Reference)\n'
        '\toptions.Filters.Competence = strings.TrimSpace(options.Filters.Competence)\n'
        '\tif options.Filters.Competence != "" && !competencePattern.MatchString(options.Filters.Competence) {\n'
        '\t\treturn ListOptions{}, ErrInvalidListOptions\n\t}',
    )
    text = text.replace(
        "case SortIdentifier, SortTypeLabel, SortBillDate, SortCreatedAt, SortUpdatedAt:",
        "case SortReference, SortTypeLabel, SortCompetence, SortAmount, SortCreatedAt, SortUpdatedAt:",
    )
    text = text.replace(
        "errors.Is(err, ErrUniquenessConflict) || errors.Is(err, ErrReferenceNotFound) ||",
        "errors.Is(err, ErrCurrentUseUnsupported) || errors.Is(err, ErrReferenceNotFound) ||",
    )
    write("internal/bill/service.go", text)


def generate_service_test() -> None:
    text = to_bill(read("internal/document/service_test.go"))
    text = text.replace("UniquenessPolicy: UniquenessNone", "SupportsCurrentUse: true")
    text = text.replace(
        'Filters{OwnerProfileID: &owner, Identifier: "  00AB  ", Status: StatusAvailable}',
        'Filters{OwnerProfileID: &owner, Reference: "  REF-00AB  ", Competence: "2026-07", Status: StatusAvailable}',
    )
    text = text.replace(
        'store.billOptions.Filters.Identifier != "00ab"',
        'store.billOptions.Filters.Reference != "ref-00ab" || store.billOptions.Filters.Competence != "2026-07"',
    )
    text = text.replace("docID", "billID")
    write("internal/bill/service_test.go", text)


def generate_audit() -> None:
    write("internal/bill/audit_store.go", to_bill(read("internal/document/audit_store.go")))
    write("internal/bill/audit_store_test.go", to_bill(read("internal/document/audit_store_test.go")))
    migration = to_bill(read("database/migrations/006_document_api_audit.sql"))
    migration = migration.replace("M4.2", "M4.3").replace("schema.bill_api', 'm4.2", "schema.bill_api', 'm4.3")
    write("database/migrations/007_bill_api_audit.sql", migration)
    write("database/queries/bill_audit.sql", to_bill(read("database/queries/document_audit.sql")))
    write("database/schema_bill_audit.sql", to_bill(read("database/schema_document_audit.sql")))


def patch_auth_sqlc() -> None:
    path = "internal/auth/auth.go"
    text = read(path)
    anchor = "func (role Role) CanManageDocumentCurrentUse() bool {\n\treturn role.Valid()\n}\n"
    addition = anchor + '''
func (role Role) CanReadBills() bool {
	return role.Valid()
}

func (role Role) CanWriteBills() bool {
	return role.Valid()
}

func (role Role) CanDeleteBills() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageBillTypes() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageBillCurrentUse() bool {
	return role.Valid()
}
'''
    write(path, replace_once(text, anchor, addition))

    path = "database/sqlc.yaml"
    text = read(path)
    write(path, replace_once(text, "      - schema_document_audit.sql\n", "      - schema_document_audit.sql\n      - schema_bill_audit.sql\n"))


def patch_composition() -> None:
    path = "cmd/api/main.go"
    text = read(path)
    text = replace_once(text, '"github.com/Pherlsz/Gymkhana-Database/internal/auth"\n', '"github.com/Pherlsz/Gymkhana-Database/internal/auth"\n\t"github.com/Pherlsz/Gymkhana-Database/internal/bill"\n')
    text = replace_once(text, "\tvar documentService *document.Service\n", "\tvar documentService *document.Service\n\tvar billService *bill.Service\n")
    anchor = '''		documentService, err = document.NewService(document.NewPostgresStore(pool), document.ServiceOptions{OnAuditFailure: func(_ context.Context, event document.AuditEvent, auditErr error) {
			logger.Error("document audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure document service: %w", err)
		}
'''
    addition = anchor + '''		billService, err = bill.NewService(bill.NewPostgresStore(pool), bill.ServiceOptions{OnAuditFailure: func(_ context.Context, event bill.AuditEvent, auditErr error) {
			logger.Error("bill audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure bill service: %w", err)
		}
'''
    text = replace_once(text, anchor, addition)
    text = replace_once(text, "Document: documentService, SecureCookies:", "Document: documentService, Bill: billService, SecureCookies:")
    write(path, text)

    path = "internal/platform/httpserver/server.go"
    text = read(path)
    text = replace_once(text, "\tDocument       documentService\n", "\tDocument       documentService\n\tBill           billService\n")
    text = replace_once(text, "\tregisterDocumentRoutes(mux, logger, settings.Auth, settings.Document)\n", "\tregisterDocumentRoutes(mux, logger, settings.Auth, settings.Document)\n\tregisterBillRoutes(mux, logger, settings.Auth, settings.Bill)\n")
    write(path, text)


def generate_http() -> None:
    source = to_bill(read("internal/platform/httpserver/document.go"))
    start = source.index("type billTypeValuesRequest struct")
    end = source.index("func registerBillRoutes")
    structs = '''type billTypeValuesRequest struct {
	TechnicalKey       string `json:"technical_key"`
	Label              string `json:"label"`
	Active             bool   `json:"active"`
	SupportsCurrentUse bool   `json:"supports_current_use"`
}

type updateBillTypeRequest struct {
	TechnicalKey       string `json:"technical_key"`
	Label              string `json:"label"`
	Active             bool   `json:"active"`
	SupportsCurrentUse bool   `json:"supports_current_use"`
	Version            int64  `json:"version"`
}

type billValuesRequest struct {
	OwnerProfileID    string           `json:"owner_profile_id"`
	BillTypeID        string           `json:"bill_type_id"`
	PrintedHolderName string           `json:"printed_holder_name"`
	PrintedAddress    string           `json:"printed_address"`
	Reference         string           `json:"reference_value"`
	Competence        string           `json:"competence"`
	Amount            string           `json:"amount"`
	Currency          string           `json:"currency"`
	Notes             string           `json:"notes"`
	RecordState       bill.RecordState `json:"record_state"`
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
	RecordState       bill.RecordState `json:"record_state"`
	Version           int64            `json:"version"`
}

type deleteBillResourceRequest struct { Version int64 `json:"version"`; Confirmation string `json:"confirmation"` }
type assignBillCurrentUseRequest struct { HolderProfileID string `json:"holder_profile_id"` }

type billTypeResponse struct {
	ID string `json:"id"`; TechnicalKey string `json:"technical_key"`; Label string `json:"label"`
	Active bool `json:"active"`; SupportsCurrentUse bool `json:"supports_current_use"`
	Version int64 `json:"version"`; CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
type billCurrentUseResponse struct { HolderProfileID string `json:"holder_profile_id"`; AssignedAt time.Time `json:"assigned_at"` }
type billResponse struct {
	ID string `json:"id"`; OwnerProfileID string `json:"owner_profile_id"`; BillTypeID string `json:"bill_type_id"`
	PrintedHolderName string `json:"printed_holder_name"`; PrintedAddress string `json:"printed_address"`; Reference string `json:"reference_value"`
	Competence string `json:"competence"`; Amount string `json:"amount"`; Currency string `json:"currency"`; Notes string `json:"notes"`
	RecordState bill.RecordState `json:"record_state"`; Status bill.Status `json:"status"`; Type billTypeResponse `json:"type"`
	CurrentUse *billCurrentUseResponse `json:"current_use"`; Version int64 `json:"version"`; CreatedAt time.Time `json:"created_at"`; UpdatedAt time.Time `json:"updated_at"`
}
type billPageMeta struct { Total int64 `json:"total"`; Limit int32 `json:"limit"`; Offset int32 `json:"offset"`; SortField string `json:"sort_field"`; SortOrder string `json:"sort_order"` }
type billTypePageResponse struct { Types []billTypeResponse `json:"types"`; Page billPageMeta `json:"page"` }
type billPageResponse struct { Bills []billResponse `json:"bills"`; Page billPageMeta `json:"page"` }

'''
    source = source[:start] + structs + source[end:]
    source = source.replace("O identificador da conta é inválido", "O identificador da conta/comprovante é inválido")
    source = source.replace("O identificador do tipo de conta é inválido", "O identificador do tipo de conta/comprovante é inválido")
    write("internal/platform/httpserver/bill.go", source)

    helper = to_bill(read("internal/platform/httpserver/document_helpers.go"))
    start = helper.index("func (request billTypeValuesRequest) domainValues()")
    end = helper.index("func parseBillIdentifier")
    custom = '''func (request billTypeValuesRequest) domainValues() bill.TypeValues {
	return bill.TypeValues{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, SupportsCurrentUse: request.SupportsCurrentUse}
}
func (request updateBillTypeRequest) domainValues() bill.TypeValues {
	return billTypeValuesRequest{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, SupportsCurrentUse: request.SupportsCurrentUse}.domainValues()
}
func (request billValuesRequest) domainValues() (bill.Values, *Problem) {
	owner, err := profile.ParseIdentifier(request.OwnerProfileID)
	if err != nil { return bill.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa proprietária é inválido"} }
	typeID, err := bill.ParseIdentifier(request.BillTypeID)
	if err != nil { return bill.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador do tipo de conta/comprovante é inválido"} }
	return bill.Values{OwnerProfileID: owner, TypeID: typeID, PrintedHolderName: request.PrintedHolderName, PrintedAddress: request.PrintedAddress, Reference: request.Reference, Competence: request.Competence, Amount: request.Amount, Currency: request.Currency, Notes: request.Notes, RecordState: request.RecordState}, nil
}
func (request updateBillRequest) domainValues() (bill.Values, *Problem) {
	return billValuesRequest{OwnerProfileID: request.OwnerProfileID, BillTypeID: request.BillTypeID, PrintedHolderName: request.PrintedHolderName, PrintedAddress: request.PrintedAddress, Reference: request.Reference, Competence: request.Competence, Amount: request.Amount, Currency: request.Currency, Notes: request.Notes, RecordState: request.RecordState}.domainValues()
}
func billTypeFromDomain(value bill.TypeDefinition) billTypeResponse {
	return billTypeResponse{ID: value.ID.String(), TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, Active: value.Values.Active, SupportsCurrentUse: value.Values.SupportsCurrentUse, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func billFromDomain(value bill.Bill) billResponse {
	response := billResponse{ID: value.ID.String(), OwnerProfileID: value.Values.OwnerProfileID.String(), BillTypeID: value.Values.TypeID.String(), PrintedHolderName: value.Values.PrintedHolderName, PrintedAddress: value.Values.PrintedAddress, Reference: value.Values.Reference, Competence: value.Values.Competence, Amount: value.Values.Amount, Currency: value.Values.Currency, Notes: value.Values.Notes, RecordState: value.Values.RecordState, Status: value.Status, Type: billTypeFromDomain(value.Type), Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.CurrentUse != nil { response.CurrentUse = &billCurrentUseResponse{HolderProfileID: value.CurrentUse.HolderProfileID.String(), AssignedAt: value.CurrentUse.AssignedAt} }
	return response
}

'''
    helper = helper[:start] + custom + helper[end:]
    old = 'Filters: bill.Filters{OwnerProfileID: owner, TypeID: typeID, Identifier: r.URL.Query().Get("identifier"), RecordState: bill.RecordState(r.URL.Query().Get("record_state")), Status: bill.Status(r.URL.Query().Get("status")), HolderProfileID: holder}'
    new = 'Filters: bill.Filters{OwnerProfileID: owner, TypeID: typeID, Reference: r.URL.Query().Get("reference"), Competence: r.URL.Query().Get("competence"), RecordState: bill.RecordState(r.URL.Query().Get("record_state")), Status: bill.Status(r.URL.Query().Get("status")), HolderProfileID: holder}'
    helper = replace_once(helper, old, new)
    helper = helper.replace('case errors.Is(err, bill.ErrUniquenessConflict):\n\t\twriteProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Já existe uma conta com esse identificador para a política configurada"})\n', '')
    anchor = 'case errors.Is(err, bill.ErrTypeInactive):\n\t\twriteProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo de conta está inativo"})\n'
    replacement = 'case errors.Is(err, bill.ErrTypeInactive):\n\t\twriteProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo de conta/comprovante está inativo"})\n\tcase errors.Is(err, bill.ErrCurrentUseUnsupported):\n\t\twriteProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Este tipo de conta/comprovante não permite uso atual"})\n'
    helper = replace_once(helper, anchor, replacement)
    helper = helper.replace("O filtro de tipo de conta é inválido", "O filtro de tipo de conta/comprovante é inválido")
    write("internal/platform/httpserver/bill_helpers.go", helper)


def generate_http_test() -> None:
    text = to_bill(read("internal/platform/httpserver/document_test.go"))
    text = text.replace("UniquenessPolicy: bill.UniquenessPerProfile", "SupportsCurrentUse: true")
    text = text.replace('bill.Values{OwnerProfileID: ownerID, TypeID: typeID, Identifier: "00AB-009", BillDate: "2026-07-15", RecordState: bill.RecordCurrent}', 'bill.Values{OwnerProfileID: ownerID, TypeID: typeID, PrintedHolderName: "Ana", PrintedAddress: "Rua A, 10", Reference: "UC-009", Competence: "2026-07", Amount: "123.45", Currency: "BRL", RecordState: bill.RecordCurrent}')
    text = text.replace("SortIdentifier", "SortReference")
    text = text.replace("&identifier=00AB", "&reference=UC&competence=2026-07")
    text = text.replace('service.billList.Filters.Identifier != "00AB"', 'service.billList.Filters.Reference != "UC" || service.billList.Filters.Competence != "2026-07"')
    text = text.replace('`{"owner_profile_id":"` + ownerID.String() + `","bill_type_id":"` + typeID.String() + `","identifier_value":"00AB-009","bill_date":"2026-07-15","notes":"","record_state":"CURRENT"}`', '`{"owner_profile_id":"` + ownerID.String() + `","bill_type_id":"` + typeID.String() + `","printed_holder_name":"Ana","printed_address":"Rua A, 10","reference_value":"UC-009","competence":"2026-07","amount":"123.45","currency":"BRL","notes":"","record_state":"CURRENT"}`')
    text = text.replace('service.billValues.Identifier != "00AB-009"', 'service.billValues.Reference != "UC-009" || service.billValues.Amount != "123.45"')
    text = text.replace('`{"technical_key":"rg","label":"RG","active":true,"uniqueness_policy":"PER_PROFILE","validation_regex":"^[A-Z0-9-]+$","date_required":true}`', '`{"technical_key":"energia","label":"Energia","active":true,"supports_current_use":true}`')
    text = text.replace('service.typeValues.TechnicalKey != "rg" || !service.typeValues.DateRequired', 'service.typeValues.TechnicalKey != "energia" || !service.typeValues.SupportsCurrentUse')
    text = text.replace('Fields: []bill.FieldError{{Field: "identifier_value", Code: "required"}}', 'Fields: []bill.FieldError{{Field: "amount", Code: "invalid_format"}}')
    text = text.replace('`{"owner_profile_id":"invalid","bill_type_id":"invalid","identifier_value":"","bill_date":"","notes":"","record_state":"CURRENT"}`', '`{"owner_profile_id":"invalid","bill_type_id":"invalid","printed_holder_name":"","printed_address":"","reference_value":"","competence":"","amount":"","currency":"","notes":"","record_state":"CURRENT"}`')
    text = text.replace('`{"technical_key":"rg","label":"","active":true,"uniqueness_policy":"NONE","validation_regex":"","date_required":false}`', '`{"technical_key":"energia","label":"","active":true,"supports_current_use":true}`')
    write("internal/platform/httpserver/bill_test.go", text)


def patch_changelog() -> None:
    path = "CHANGELOG.md"
    text = read(path)
    anchor = "### Added\n\n"
    bullets = (
        "- Protected bill-type administration and Profile-owned bill CRUD API with centralized role permissions.\n"
        "- Bill filtering, sorting, pagination, structured validation, optimistic conflicts, duplication, and explicit permanent-delete confirmation.\n"
        "- Printed holder/address/reference preservation with civil competence, canonical decimal amounts, and ISO currency validation.\n"
        "- Current-use assignment/replacement/return only for bill types that explicitly support it.\n"
        "- Durable bill mutation audit events, OpenAPI 0.6.0 contracts, generated clients, and service/HTTP/PostgreSQL coverage.\n"
    )
    write(path, replace_once(text, anchor, anchor + bullets))


def main() -> None:
    generate_service()
    generate_service_test()
    generate_audit()
    patch_auth_sqlc()
    patch_composition()
    generate_http()
    generate_http_test()
    patch_changelog()


if __name__ == "__main__":
    main()
